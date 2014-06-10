// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"fmt"
	"net/http"
	"sync"
)

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

var (
	mutex             sync.Mutex
	sessionWrapperMap map[string]*sessionWrapper
)

type sessionWrapper struct {
	Session
	si          *SessionInfo
	reqNotifier chan *reqRegister
}

type reqRegister struct {
	w    http.ResponseWriter
	r    *http.Request
	done chan struct{}
}

func newReqRegister(w http.ResponseWriter, r *http.Request) *reqRegister {
	return &reqRegister{w, r, make(chan struct{})}
}

func init() {
	sessionWrapperMap = make(map[string]*sessionWrapper)
}

func debug(format string, a ...interface{}) {
	sm.Debug(fmt.Sprintf(format, a...))
}

func newSession(r *http.Request) (*sessionWrapper, error) {
	mutex.Lock()
	defer mutex.Unlock()
	session, err := sm.NewSession(r)
	if err != nil {
		return nil, err
	}

	sw := &sessionWrapper{
		session,
		&SessionInfo{-1, 0, -1},
		make(chan *reqRegister),
	}
	launchSession(sw)

	sessionWrapperMap[session.SID()] = sw
	return sw, nil
}

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

	sw := &sessionWrapper{session, si, make(chan *reqRegister)}
	launchSession(sw)

	sessionWrapperMap[sid] = sw
	return sw, nil
}

// BindHandler handles forward and backward channel HTTP requests. When using
// the defaults this handler should be installed at "/channel" (WebChannel) or
// "/channel/bind" (BrowserChannel).
func BindHandler(w http.ResponseWriter, r *http.Request) {
	if sm == nil {
		panic("No SessionManager provided")
	}
	var sw *sessionWrapper
	var err error
	switch {
	case r.FormValue("SID") == "":
		sw, err = newSession(r)
	default:
		sw, err = getSession(r)
	}
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
	rr := newReqRegister(w, r)
	sw.reqNotifier <- rr
	<-rr.done
}
