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

func shouldNOOP(br *backChannelRegister) bool {
	return br != nil && br.r.FormValue("CI") != "1"
}

func flushPending(p *padder, msgs []*Message) error {
	// TODO(ahochhaus)
	return nil
}

func sessionWorker(sw *sessionWrapper) {
	noopTimer := time.NewTimer(30 * time.Second)
	noopTimer.Stop()
	longBackChannelTimer := time.NewTimer(4 * 60 * time.Second)
	longBackChannelTimer.Stop()
	var br *backChannelRegister
	var backChannelCloseNotifier <-chan bool
	var p *padder

	for {
		select {
		case <-noopTimer.C:
			if shouldNOOP(br) {
				if sw.si.BachChannelBytes == 0 {
					// if a non-buffered, active backchannel w/o pending data add noop
					err := sw.BackChannelAdd([]byte("[\"noop\"]"))
					if err != ErrDropTransientMessage {
						sm.Error(br.r, err)
					}
				}
				noopTimer.Reset(30 * time.Second)
			}
		case <-longBackChannelTimer.C:
			if br != nil {
				p.end()
				sw.BackChannelClose()
				close(br.done)
			}
			br = nil
			p = nil
			backChannelCloseNotifier = nil
			noopTimer.Stop()
			longBackChannelTimer.Stop()
		case <-backChannelCloseNotifier:
			if br != nil {
				sw.BackChannelClose()
				close(br.done)
			}
			br = nil
			p = nil
			backChannelCloseNotifier = nil
			noopTimer.Stop()
			longBackChannelTimer.Stop()
		case <-sw.clientTerminateNotifier:
			if br != nil {
				sw.BackChannelClose()
				close(br.done)
			}
			break
		case tempBR := <-sw.newBackChannelNotifier:
			if br != nil {
				sw.BackChannelClose()
				sm.Error(tempBR.r, errors.New("Duplicate backchannel."))
				close(br.done)
			}
			// TODO(hochhaus): flush pending messages
			noopTimer.Reset(30 * time.Second)
			longBackChannelTimer.Reset(4 * 60 * time.Second)
			br = tempBR
			p = newPadder(tempBR.w, tempBR.r)
			cn, ok := br.w.(http.CloseNotifier)
			if !ok {
				panic("webserver doesn't support close notification")
			}
			backChannelCloseNotifier = cn.CloseNotify()
			sw.BackChannelOpen()
		case sa := <-sw.Notifier():
			switch {
			case sa == ServerTerminate:
				if br != nil {
					// Add "stop" message directly so that calling application cannot
					// reject it.
					sw.BackChannelAdd([]byte("[\"stop\"]"))
					// TODO(hochhaus): Send "stop" message to client
					sw.BackChannelClose()
					close(br.done)
				}
				// TODO(hochhaus): cleanup session state
				break
			case sa > 0:
				// BackChannelActivity
				sw.si.BachChannelBytes += int(sa)
				if br != nil {
					// TODO(hochhaus): flush pending messages
				}
			default:
				log.Panicf("Unsupported SessionActivity: %d", sa)
			}
		}
	}
}
