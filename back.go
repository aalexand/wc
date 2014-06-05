// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"net/http"
)

type backChannelRegister struct {
	w    http.ResponseWriter
	r    *http.Request
	done chan struct{}
}

func newBackChannelRegister(
	w http.ResponseWriter,
	r *http.Request,
) *backChannelRegister {
	return &backChannelRegister{w, r, make(chan struct{})}
}

func backChannelHandler(w http.ResponseWriter, r *http.Request) {
	sessWrap, err := getSession(r)
	if err != nil {
		sm.Error(r, err)
		switch {
		case err == ErrUnknownSID:
			// Special case 'Unknown SID' to be compatible with JS impl. See
			// goog.labs.net.webChannel.ChannelRequest#onXmlHttpReadyStateChanged_
			// for more details.
			http.Error(w, ErrUnknownSID.Error(), 400)
		default:
			http.Error(w, "Unable to locate SID", http.StatusInternalServerError)
		}
		return
	}

	c := newBackChannelRegister(w, r)
	sessWrap.newBackChannelNotifier <- c
	// Block returning until backchannel is no longer used
	<-c.done
}
