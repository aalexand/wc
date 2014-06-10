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

func noop(sw *sessionWrapper, bc *reqRegister, p *padder) {
	if bc == nil || bc.r.FormValue("CI") != "1" {
		return
	}
	if sw.si.BackChannelBytes > 0 {
		return
	}

	// if a non-buffered, active backchannel w/o pending data add noop
	debug("wc: %s noop", sw.SID())
	if err := sw.BackChannelAdd([]byte("[\"noop\"]")); err != nil {
		sm.Error(bc.r, err)
		return
	}
	sw.si.BackChannelBytes += 8
	if err := flushPending(sw, p); err != nil {
		sm.Error(bc.r, err)
	}
}

func flushPending(sw *sessionWrapper, p *padder) error {
	if sw.si.BackChannelBytes <= 0 {
		return nil
	}
	msgs, err := sw.BackChannelPeek()
	if err != nil {
		return err
	}
	for _, msg := range msgs {
		debug("wc: %s writing back channel message %d %s", sw.SID(), msg.ID,
			msg.Body)
	}
	return p.chunkMessages(msgs)
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
	ackedBytes := 0
	ackedMessages := false
	for _, bcMsg := range bcMsgs {
		if bcMsg.ID > aid {
			break
		}
		ackedBytes += len(bcMsg.Body)
		ackedMessages = true
	}
	if !ackedMessages {
		return true
	}

	err = sw.BackChannelACKThrough(aid)
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to ACK back channel up to AID", 400)
		return false
	}

	sw.si.BackChannelAID = aid
	sw.si.BackChannelBytes -= ackedBytes
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
	noopTimer := time.NewTimer(30 * time.Second)
	noopTimer.Stop()
	longBackChannelTimer := time.NewTimer(4 * 60 * time.Second)
	longBackChannelTimer.Stop()
	var bc *reqRegister
	var backChannelCloseNotifier <-chan bool
	var p *padder

	for {
		select {
		case <-noopTimer.C:
			noop(sw, bc, p)
			noopTimer.Reset(30 * time.Second)

		case <-longBackChannelTimer.C:
			if bc != nil {
				debug("wc: %s closing long-lived back channel", sw.SID())
				p.end()
				sw.BackChannelClose()
				close(bc.done)
			}
			bc = nil
			p = nil
			backChannelCloseNotifier = nil
			noopTimer.Stop()
			longBackChannelTimer.Stop()

		case <-backChannelCloseNotifier:
			if bc != nil {
				debug("wc: %s back channel closed", sw.SID())
				sw.BackChannelClose()
				close(bc.done)
			}
			bc = nil
			p = nil
			backChannelCloseNotifier = nil
			noopTimer.Stop()
			longBackChannelTimer.Stop()

		case reqRequest := <-sw.reqNotifier:
			switch {
			case reqRequest.r.FormValue("TYPE") == "xmlhttp" ||
				reqRequest.r.FormValue("TYPE") == "html":
				debug("wc: %s new back channel", sw.SID())
				if !maybeACKBackChannel(sw, reqRequest.w, reqRequest.r) {
					close(bc.done)
					return
				}

				if bc != nil {
					sw.BackChannelClose()
					sm.Error(reqRequest.r, errors.New("Duplicate backchannel."))
					close(bc.done)
				}
				bc = reqRequest
				p = newPadder(reqRequest.w, reqRequest.r)
				cn, ok := bc.w.(http.CloseNotifier)
				if !ok {
					panic("webserver doesn't support close notification")
				}
				backChannelCloseNotifier = cn.CloseNotify()
				noopTimer.Reset(30 * time.Second)
				longBackChannelTimer.Reset(4 * 60 * time.Second)
				sw.BackChannelOpen()
				if err := flushPending(sw, p); err != nil {
					sm.Error(bc.r, err)
				}

			case reqRequest.r.FormValue("TYPE") == "terminate":
				debug("wc: %s client terminate session", sw.SID())
				sid := sw.SID()
				err := sm.TerminatedSession(sid, ClientTerminateRequest)
				if err != nil {
					sm.Error(reqRequest.r, err)
					http.Error(reqRequest.w, "Unable to terminate",
						http.StatusInternalServerError)
					return
				}

				if bc != nil {
					sw.BackChannelClose()
					close(bc.done)
				}

				mutex.Lock()
				delete(sessionWrapperMap, sid)
				mutex.Unlock()

				reqRequest.w.Write([]byte("Terminated"))
				break

			case reqRequest.r.FormValue("SID") == "":
				debug("wc: %s forward channel (new session)", sw.SID())
				newSessionHandler(sw, reqRequest.w, reqRequest.r)
				reqRequest.done <- struct{}{}

			default:
				debug("wc: %s forward channel", sw.SID())
				if !maybeACKBackChannel(sw, reqRequest.w, reqRequest.r) {
					return
				}
				fcHandler(sw, bc != nil, reqRequest.w, reqRequest.r)
				reqRequest.done <- struct{}{}
			}

		case sa := <-sw.Notifier():
			switch {
			case sa == ServerTerminate:
				debug("wc: %s server terminate session", sw.SID(), sa)
				if bc != nil {
					// TODO(hochhaus): persist the session termination until the client
					// ACKs it?
					msgs := []*Message{
						&Message{0, []byte(jsonArray([]interface{}{"stop"}))},
					}
					p.chunkMessages(msgs)
					p.end()
					sw.BackChannelClose()
					close(bc.done)
					bc = nil
					p = nil
					backChannelCloseNotifier = nil
					noopTimer.Stop()
					longBackChannelTimer.Stop()
				}
				break
			default:
				panic(fmt.Sprintf("Unsupported SessionActivity: %d", sa))
			}

		case sa := <-activityNotifier:
			debug("wc: %s new back channel data %d bytes", sw.SID(), sa)
			// BackChannelActivity
			sw.si.BackChannelBytes += sa
			if bc != nil {
				if err := flushPending(sw, p); err != nil {
					sm.Error(bc.r, err)
				}
			}
		}
	}
}
