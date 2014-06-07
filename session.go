// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"errors"
	"log"
	"net/http"
	"time"
)

func shouldNOOP(bc *reqRegister) bool {
	return bc != nil && bc.r.FormValue("CI") != "1"
}

func flushPending(sw *sessionWrapper, p *padder) error {
	if sw.si.BachChannelBytes == 0 {
		return nil
	}
	msgs, err := sw.BackChannelPeek()
	if err != nil {
		return err
	}
	return p.chunkMessages(msgs)
}

func terminateHandler(
	sw *sessionWrapper,
	w http.ResponseWriter,
	r *http.Request,
) {
}

func sessionWorker(sw *sessionWrapper) {
	noopTimer := time.NewTimer(30 * time.Second)
	noopTimer.Stop()
	longBackChannelTimer := time.NewTimer(4 * 60 * time.Second)
	longBackChannelTimer.Stop()
	var bc *reqRegister
	var backChannelCloseNotifier <-chan bool
	var p *padder

	for {
		log.Printf("sessionWorker: %s", sw.SID())
		select {
		case <-noopTimer.C:
			log.Printf("  %s: noopTimer", sw.SID())
			if shouldNOOP(bc) {
				if sw.si.BachChannelBytes == 0 {
					// if a non-buffered, active backchannel w/o pending data add noop
					err := sw.BackChannelAdd([]byte("[\"noop\"]"))
					if err != ErrDropTransientMessage {
						sm.Error(bc.r, err)
					}
				}
				noopTimer.Reset(30 * time.Second)
			}
		case <-longBackChannelTimer.C:
			log.Printf("  %s: longBackChannelTimer", sw.SID())
			if bc != nil {
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
			log.Printf("  %s: backChannelCloseNotifier", sw.SID())
			if bc != nil {
				sw.BackChannelClose()
				close(bc.done)
			}
			bc = nil
			p = nil
			backChannelCloseNotifier = nil
			noopTimer.Stop()
			longBackChannelTimer.Stop()
		case reqRequest := <-sw.reqNotifier:
			log.Printf("  %s: reqNotifier", sw.SID())
			switch {
			case reqRequest.r.FormValue("TYPE") == "xmlhttp" ||
				reqRequest.r.FormValue("TYPE") == "html":
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
				newSessionHandler(sw, reqRequest.w, reqRequest.r)
			default:
				fcHandler(sw, bc != nil, reqRequest.w, reqRequest.r)
			}

		case sa := <-sw.Notifier():
			log.Printf("  %s: <-sw.Notifier() -- %d", sw.SID(), sa)
			switch {
			case sa == ServerTerminate:
				if bc != nil {
					// Write ["stop"] message directly to avoid application rejecting it.
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
			case sa > 0:
				// BackChannelActivity
				sw.si.BachChannelBytes += int(sa)
				if bc != nil {
					if err := flushPending(sw, p); err != nil {
						sm.Error(bc.r, err)
					}
				}
			default:
				log.Panicf("Unsupported SessionActivity: %d", sa)
			}
		}
	}
}
