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

func sessionWorker(sw *sessionWrapper) {
	noopTimer := time.NewTimer(30 * time.Second)
	noopTimer.Stop()
	var br *backChannelRegister
	var backChannelCloseNotifier <-chan bool

	for {
		select {
		case <-noopTimer.C:
			if br != nil {
				// TODO(hochhaus: write noop message (only in non-buffered mode?)
			}
		case <-backChannelCloseNotifier:
			sw.BackChannelClose()
			close(br.done)
			br = nil
			backChannelCloseNotifier = nil
			noopTimer.Stop()
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
			br = tempBR
			cn, ok := br.w.(http.CloseNotifier)
			if !ok {
				panic("webserver doesn't support close notification")
			}
			backChannelCloseNotifier = cn.CloseNotify()
			sw.BackChannelOpen()
		case sa := <-sw.Notifier():
			switch sa {
			case BackChannelActivity:
				if br != nil {
					// TODO(hochhaus): flush pending messages
				}
			case ServerTerminate:
				if br != nil {
					// TODO(hochhaus): Send "stop" message to client
					sw.BackChannelClose()
					close(br.done)
				}
				// TODO(hochhaus): cleanup session state
				break
			default:
				log.Panicf("Unsupported SessionActivity: %d", sa)
			}
		}
	}
}
