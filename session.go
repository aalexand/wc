// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func flushPending(sw *sessionWrapper) error {
	msgs, err := sw.BackChannelPeek()
	if err != nil {
		return err
	}
	for len(msgs) > 0 {
		if msgs[0].ID > sw.si.BackChannelAID {
			break
		}
		msgs = msgs[1:]
	}
	if len(msgs) == 0 {
		return nil
	}
	for _, msg := range msgs {
		debug("wc: %s writing back channel message %d %s", sw.SID(), msg.ID,
			msg.Body)
	}
	sw.si.BackChannelAID = msgs[len(msgs)-1].ID
	err = sw.p.chunkMessages(msgs)
	if err != nil {
		return err
	}
	if sw.bc.r.FormValue("CI") == "1" {
		// TODO(hochhaus): Do not write messages in chunked mode when the back
		// channel is a buffered proxy. Just reply to the entire request at once.
		debug("wc: %s closing buffered-proxy back channel to deliver messages",
			sw.SID())
		sw.p.end()
		sw.BackChannelClose()
		close(sw.bc.done)
		sw.bc = nil
		sw.p = nil
		sw.backChannelCloseNotifier = nil
		sw.noopTimer.Stop()
		sw.longBackChannelTimer.Stop()
	}
	return nil
}

func noop(sw *sessionWrapper) {
	if sw.bc == nil {
		debug("wc: %s noop skipped", sw.SID())
		return
	}

	// if a non-buffered, active backchannel w/o pending data add noop
	debug("wc: %s noop", sw.SID())
	sw.noopTimer.Reset(30 * time.Second)

	if err := sw.BackChannelAdd([]byte("[\"noop\"]")); err != nil {
		sm.Error(sw.bc.r, err)
		return
	}
	sw.backChannelBytes += 8
	if err := flushPending(sw); err != nil {
		sm.Error(sw.bc.r, err)
	}
}

func longBackChannel(sw *sessionWrapper) {
	if sw.bc != nil {
		debug("wc: %s closing long-lived back channel", sw.SID())
		sw.p.end()
		sw.BackChannelClose()
		close(sw.bc.done)
	}
	sw.bc = nil
	sw.p = nil
	sw.backChannelCloseNotifier = nil
	sw.noopTimer.Stop()
	sw.longBackChannelTimer.Stop()
}

func backChannelClose(sw *sessionWrapper) {
	if sw.bc != nil {
		debug("wc: %s back channel closed", sw.SID())
		sw.BackChannelClose()
		close(sw.bc.done)
	}
	sw.bc = nil
	sw.p = nil
	sw.backChannelCloseNotifier = nil
	sw.noopTimer.Stop()
	sw.longBackChannelTimer.Stop()
}

func backChannel(sw *sessionWrapper, reqRequest *reqRegister) {
	debug("wc: %s new back channel", sw.SID())
	if !maybeACKBackChannel(sw, reqRequest.w, reqRequest.r, false) {
		close(sw.bc.done)
		return
	}

	if sw.bc != nil {
		sw.BackChannelClose()
		sm.Error(reqRequest.r, errors.New("Duplicate backchannel."))
		close(sw.bc.done)
	}
	sw.bc = reqRequest
	sw.p = newPadder(reqRequest.w, reqRequest.r)
	cn, ok := reqRequest.w.(http.CloseNotifier)
	if !ok {
		panic("webserver doesn't support close notification")
	}
	sw.backChannelCloseNotifier = cn.CloseNotify()
	sw.noopTimer.Reset(30 * time.Second)
	sw.longBackChannelTimer.Reset(4 * 60 * time.Second)
	sw.BackChannelOpen()
	if err := flushPending(sw); err != nil {
		sm.Error(sw.bc.r, err)
	}
}

func clientTerminate(sw *sessionWrapper, reqRequest *reqRegister) {
	debug("wc: %s client terminate session", sw.SID())
	defer func() {
		reqRequest.done <- struct{}{}
	}()

	err := sm.TerminatedSession(sw.Session, ClientTerminateRequest)
	if err != nil {
		sm.Error(reqRequest.r, err)
		http.Error(reqRequest.w, "Unable to terminate",
			http.StatusInternalServerError)
		return
	}

	if sw.bc != nil {
		sw.BackChannelClose()
		close(sw.bc.done)
		sw.bc = nil
		sw.p = nil
		sw.backChannelCloseNotifier = nil
		sw.noopTimer.Stop()
		sw.longBackChannelTimer.Stop()
	}

	mutex.Lock()
	delete(sessionWrapperMap, sw.SID())
	mutex.Unlock()

	reqRequest.w.Write([]byte("Terminated"))
}

func launchSession(sw *sessionWrapper) {
	activityNotifier := make(chan int)
	go sessionWorker(sw, activityNotifier)
	go activityProxyWorker(sw, activityNotifier)
}

func maybeACKBackChannel(
	sw *sessionWrapper,
	w http.ResponseWriter,
	r *http.Request,
	forwardChannel bool,
) bool {
	aid, err := strconv.Atoi(r.FormValue("AID"))
	if err != nil || aid < 0 {
		sm.Error(r, err)
		http.Error(w, "Unable to parse AID", 400)
		return false
	}
	bcMsgs, err := sw.BackChannelPeek()
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to get messages", http.StatusInternalServerError)
		return false
	}
	remainingBytes := 0
	ackedBytes := 0
	messagesToACK := false
	for _, bcMsg := range bcMsgs {
		if bcMsg.ID > aid {
			remainingBytes += len(bcMsg.Body)
		} else {
			messagesToACK = true
			ackedBytes += len(bcMsg.Body)
		}
	}
	if !messagesToACK {
		if !forwardChannel {
			// Retransmit any un-ACKed messages on the new back channel.
			sw.si.BackChannelAID = aid
			sw.backChannelBytes = remainingBytes
		}
		return true
	}

	err = sw.BackChannelACKThrough(aid)
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to ACK back channel up to AID", 400)
		return false
	}
	if forwardChannel {
		// Do not trigger retransmit on the current back channel
		sw.backChannelBytes -= ackedBytes
	} else {
		// Retransmit any un-ACKed messages on the new back channel.
		sw.si.BackChannelAID = aid
		sw.backChannelBytes = remainingBytes
	}
	debug("wc: %s ACKed back channel %d bytes up to AID %d", sw.SID(),
		ackedBytes, aid)
	return true
}

func activityProxyWorker(sw *sessionWrapper, activityNotifier chan int) {
	var an chan int
	var proxiedByteCount int
	for {
		// TODO(hochhaus): shutdown activityProxyWorker when the corresponding
		// session has been terminated.
		select {
		case i := <-sw.DataNotifier():
			debug("wc: %s new back channel data %d bytes (proxied)", sw.SID(), i)
			proxiedByteCount += i
			if proxiedByteCount > 0 {
				an = activityNotifier
			}
		case an <- proxiedByteCount:
			debug("wc: %s new back channel data %d bytes (non-proxied)", sw.SID(),
				proxiedByteCount)
			proxiedByteCount = 0
			an = nil
		}
	}
}

func sessionWorker(sw *sessionWrapper, activityNotifier chan int) {
	for {
		select {
		case <-sw.noopTimer.C:
			noop(sw)
		case <-sw.longBackChannelTimer.C:
			longBackChannel(sw)
		case <-sw.backChannelCloseNotifier:
			backChannelClose(sw)
		case reqRequest := <-sw.reqNotifier:
			switch {
			case reqRequest.r.FormValue("TYPE") == "xmlhttp" ||
				reqRequest.r.FormValue("TYPE") == "html":
				backChannel(sw, reqRequest)
			case reqRequest.r.FormValue("TYPE") == "terminate":
				clientTerminate(sw, reqRequest)
			case reqRequest.r.FormValue("SID") == "":
				newSessionHandler(sw, reqRequest)
			default:
				fcHandler(sw, reqRequest)
			}

		case sa := <-sw.Notifier():
			switch {
			case sa == ServerTerminate:
				debug("wc: %s server terminate session", sw.SID(), sa)
				err := sm.TerminatedSession(sw.Session, ServerTerminateRequest)
				if err != nil {
					sm.Error(sw.bc.r, err)
				}
				if sw.bc != nil {
					// TODO(hochhaus): persist the session termination until the client
					// ACKs it?
					msgs := []*Message{
						&Message{0, []byte(jsonArray([]interface{}{"stop"}))},
					}
					sw.p.chunkMessages(msgs)
					sw.p.end()
					sw.BackChannelClose()
					close(sw.bc.done)
					sw.bc = nil
					sw.p = nil
					sw.backChannelCloseNotifier = nil
					sw.noopTimer.Stop()
					sw.longBackChannelTimer.Stop()
				}
				break
			default:
				panic(fmt.Sprintf("Unsupported SessionActivity: %d", sa))
			}

		case sa := <-activityNotifier:
			debug("wc: %s new back channel data %d bytes", sw.SID(), sa)
			// BackChannelActivity
			sw.backChannelBytes += sa
			if sw.bc != nil {
				if err := flushPending(sw); err != nil {
					sm.Error(sw.bc.r, err)
				}
			}
		}
	}
}
