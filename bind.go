// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"net/http"
	"sync"
)

var (
	mutex             sync.Mutex
	sessionWrapperMap map[string]*sessionWrapper
)

type sessionWrapper struct {
	Session
	newBackChannelNotifier  chan *backChannelRegister
	clientTerminateNotifier chan struct{}
}

// TODO(hochhaus): Invoke BackChannelClose using (http.CloseNotifier)

// Special cases to think about:
// * duplicate back channel
// * noop message on back channel
// * buffered proxy mode (ensure flushing all messages)
// * 'Unknown SID' error message with HTTP status 400
// * other ChannelRequest.Error error values (special cases?)
// * XSS escaping messages for client
// * backchannel handoff
// * messages delivered when no back channel exists for a session
// * client side session reconnects after server crash
// * chunk compression

// Tasks for application level:
// * sharding comet server? (moving sessions across servers?)
// * restart server without dropping back channels
// * expvar stats: # sessions, # backchannels, # pending messages, etc

// message SessionBuffer {
//   optional string sid = 1;
//   repeated string buffer = 2;
//   optional uint64 buffer_start_array_id = 3;
//   optional uint64 cur_back_channel_flushed_index = 4;
//   optional uint64 outstanding_back_channel_bytes = 5;
//   optional PaddingType padding = 6;
// }

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

func getSession(w http.ResponseWriter, r *http.Request) (
	*sessionWrapper,
	error,
) {
	mutex.Lock()
	defer mutex.Unlock()
	sid := r.FormValue("SID")
	if sessionWrapper, hasSession := sessionWrapperMap[sid]; hasSession {
		return sessionWrapper, nil
	}
	session, err := sm.LookupSession(sid)
	if err != nil {
		return nil, err
	}

	sessionWrapper := &sessionWrapper{
		session,
		make(chan *backChannelRegister),
		make(chan struct{}),
	}
	go sessionWorker(sessionWrapper)

	sessionWrapperMap[sid] = sessionWrapper
	return sessionWrapper, nil
}

func backChannelHandler(w http.ResponseWriter, r *http.Request) {
	sessionWrapper, err := getSession(w, r)
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
	sessionWrapper.newBackChannelNotifier <- c
	// Block returning until backchannel is no longer used
	<-c.done
}

func terminateHandler(w http.ResponseWriter, r *http.Request) {
	sessionWrapper, err := getSession(w, r)
	switch {
	case err == ErrUnknownSID:
		w.Write([]byte("Terminated"))
		return
	case err != nil:
		sm.Error(r, err)
		http.Error(w, "Unable to locate SID", http.StatusInternalServerError)
		return
	}
	sid := sessionWrapper.SID()
	err = sm.TerminatedSession(sid, ClientTerminateRequest)
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to terminate", http.StatusInternalServerError)
		return
	}

	close(sessionWrapper.clientTerminateNotifier)

	mutex.Lock()
	delete(sessionWrapperMap, sid)
	mutex.Unlock()

	w.Write([]byte("Terminated"))
}

func forwardChannelHandler(w http.ResponseWriter, r *http.Request) {
}

// BindHandler handles forward and backward channel HTTP requests. When using
// the defaults this handler should be installed at "/channel" (WebChannel) or
// "/channel/bind" (BrowserChannel).
func BindHandler(w http.ResponseWriter, r *http.Request) {
	if sm == nil {
		panic("No SessionManager provided")
	}
	switch r.FormValue("TYPE") {
	case "xmlhttp", "html":
		backChannelHandler(w, r)
	case "terminate":
		terminateHandler(w, r)
	default:
		forwardChannelHandler(w, r)
	}
}
