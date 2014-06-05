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
	si                      *SessionInfo
	newBackChannelNotifier  chan *backChannelRegister
	clientTerminateNotifier chan struct{}
	backChannelPresent      bool
}

func init() {
	sessionWrapperMap = make(map[string]*sessionWrapper)
}

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

func getSession(r *http.Request) (*sessionWrapper, error) {
	mutex.Lock()
	defer mutex.Unlock()
	sid := r.FormValue("SID")
	if sw, hasSession := sessionWrapperMap[sid]; hasSession {
		return sw, nil
	}
	session, si, err := sm.LookupSession(sid)
	if err != nil {
		return nil, err
	}

	sw := &sessionWrapper{
		session,
		si,
		make(chan *backChannelRegister),
		make(chan struct{}),
		false,
	}
	go sessionWorker(sw)

	sessionWrapperMap[sid] = sw
	return sw, nil
}

func terminateHandler(w http.ResponseWriter, r *http.Request) {
	sw, err := getSession(r)
	switch {
	case err == ErrUnknownSID:
		w.Write([]byte("Terminated"))
		return
	case err != nil:
		sm.Error(r, err)
		http.Error(w, "Unable to locate SID", http.StatusInternalServerError)
		return
	}
	sid := sw.SID()
	err = sm.TerminatedSession(sid, ClientTerminateRequest)
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to terminate", http.StatusInternalServerError)
		return
	}

	close(sw.clientTerminateNotifier)

	mutex.Lock()
	delete(sessionWrapperMap, sid)
	mutex.Unlock()

	w.Write([]byte("Terminated"))
}

// BindHandler handles forward and backward channel HTTP requests. When using
// the defaults this handler should be installed at "/channel" (WebChannel) or
// "/channel/bind" (BrowserChannel).
func BindHandler(w http.ResponseWriter, r *http.Request) {
	if sm == nil {
		panic("No SessionManager provided")
	}
	switch {
	case r.FormValue("TYPE") == "xmlhttp" || r.FormValue("TYPE") == "html":
		backChannelHandler(w, r)
	case r.FormValue("TYPE") == "terminate":
		terminateHandler(w, r)
	case r.FormValue("SID") == "":
		newSessionHandler(w, r)
	default:
		forwardChannelHandler(w, r)
	}
}
