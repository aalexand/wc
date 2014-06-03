// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

func newSession(r *http.Request) (*sessionWrapper, error) {
	mutex.Lock()
	defer mutex.Unlock()
	session, err := sm.NewSession(r)
	if err != nil {
		return nil, err
	}

	sessWrap := &sessionWrapper{
		session,
		&SessionInfo{-1, 0, -1},
		make(chan *backChannelRegister),
		make(chan struct{}),
		false,
	}
	go sessionWorker(sessWrap)

	sessionWrapperMap[session.SID()] = sessWrap
	return sessWrap, nil
}

func getSession(r *http.Request) (*sessionWrapper, error) {
	mutex.Lock()
	defer mutex.Unlock()
	sid := r.FormValue("SID")
	if sessWrap, hasSession := sessionWrapperMap[sid]; hasSession {
		return sessWrap, nil
	}
	session, si, err := sm.LookupSession(sid)
	if err != nil {
		return nil, err
	}

	sessWrap := &sessionWrapper{
		session,
		si,
		make(chan *backChannelRegister),
		make(chan struct{}),
		false,
	}
	go sessionWorker(sessWrap)

	sessionWrapperMap[sid] = sessWrap
	return sessWrap, nil
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

func terminateHandler(w http.ResponseWriter, r *http.Request) {
	sessWrap, err := getSession(r)
	switch {
	case err == ErrUnknownSID:
		w.Write([]byte("Terminated"))
		return
	case err != nil:
		sm.Error(r, err)
		http.Error(w, "Unable to locate SID", http.StatusInternalServerError)
		return
	}
	sid := sessWrap.SID()
	err = sm.TerminatedSession(sid, ClientTerminateRequest)
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to terminate", http.StatusInternalServerError)
		return
	}

	close(sessWrap.clientTerminateNotifier)

	mutex.Lock()
	delete(sessionWrapperMap, sid)
	mutex.Unlock()

	w.Write([]byte("Terminated"))
}

func forwardChannelHandler(w http.ResponseWriter, r *http.Request) {
	var sessWrap *sessionWrapper
	var err error
	if r.FormValue("SID") == "" {
		sessWrap, err = newSession(r)
		if err != nil {
			sm.Error(r, err)
			http.Error(w, "Unable to create SID", http.StatusInternalServerError)
			return
		}
		// TODO(hochhaus): Add message ID = 0 with value
		// ["c","5432123456789012","b",8]
		// ["c",sessionId,hostPrefix_,negotiatedVersion]
	} else {
		sessWrap, err = getSession(r)
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
	}

	count, err := strconv.Atoi(r.PostFormValue("count"))
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to parse count", 400)
		return
	}
	offset, err := strconv.Atoi(r.PostFormValue("ofs"))
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to parse ofs", 400)
		return
	}

	msgs := []*Message{}
	for i := 0; i < count; i++ {
		req := fmt.Sprintf("req%d", i)
		jsonMap := make(map[string]interface{})
		for key, value := range r.PostForm {
			keyParts := strings.SplitN(key, "_", 2)
			if len(keyParts) < 2 || keyParts[0] != req {
				continue
			}
			jsonMap[keyParts[1]] = value
		}
		effectiveID := offset + i
		if effectiveID <= sessWrap.si.ForwardChannelAID {
			// skip incoming messages which have already been received
			continue
		}
		msgs = append(msgs, &Message{ID: offset + i, Body: jsonObject(jsonMap)})
	}

	if len(msgs) > 0 {
		err = sessWrap.ForwardChannel(msgs)
		if err != nil {
			sm.Error(r, err)
			http.Error(w, "Incoming message error", http.StatusInternalServerError)
			return
		}
	}

	if r.FormValue("SID") == "" {
		// TODO(hochhaus): No back channel, so we must flush to forward channel.
	} else {
		reply := []interface{}{
			sessWrap.backChannelPresent,
			sessWrap.si.BackChannelAID,
			sessWrap.si.BachChannelBytes,
		}
		// TODO(ahochhaus): Do not write using chunk() interface
		p := newPadder(w, r)
		p.chunk(jsonArray(reply))
		p.end()
	}
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
