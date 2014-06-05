// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

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
		make(chan *backChannelRegister),
		make(chan struct{}),
		false,
	}
	go sessionWorker(sw)

	sessionWrapperMap[session.SID()] = sw
	return sw, nil
}

func newSessionHandler(w http.ResponseWriter, r *http.Request) {
	sw, err := newSession(r)
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to create SID", http.StatusInternalServerError)
		return
	}

	msgs := []*Message{
		// create session message: ["c",sessionId,hostPrefix_,negotiatedVersion]
		&Message{0, []byte(jsonArray(
			[]interface{}{"c", sw.SID(), sm.HostPrefix(), 8},
		))},
	}

	if sw.si.BachChannelBytes > 0 {
		peekMsgs, err := sw.BackChannelPeek()
		if err != nil {
			sm.Error(r, err)
			http.Error(w, "Unable to get messages", http.StatusInternalServerError)
			return
		}
		for _, msg := range peekMsgs {
			msgs = append(msgs, msg)
		}
	}

	p := newPadder(w, r)
	p.writeMessages(msgs)
}

func forwardChannelHandler(w http.ResponseWriter, r *http.Request) {
	sw, err := getSession(r)
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

	count, err := strconv.Atoi(r.PostFormValue("count"))
	if err != nil {
		sm.Error(r, err)
		http.Error(w, "Unable to parse count", 400)
		return
	}

	msgs := []*Message{}
	if count > 0 {
		offset, err := strconv.Atoi(r.PostFormValue("ofs"))
		if err != nil {
			sm.Error(r, err)
			http.Error(w, "Unable to parse ofs", 400)
			return
		}

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
			if effectiveID <= sw.si.ForwardChannelAID {
				// skip incoming messages which have already been received
				continue
			}
			msgs = append(msgs,
				&Message{ID: offset + i, Body: []byte(jsonObject(jsonMap))})
		}
	}

	if len(msgs) > 0 {
		err = sw.ForwardChannel(msgs)
		if err != nil {
			sm.Error(r, err)
			http.Error(w, "Incoming message error", http.StatusInternalServerError)
			return
		}
	}

	reply := []interface{}{
		sw.backChannelPresent,
		sw.si.BackChannelAID,
		sw.si.BachChannelBytes,
	}
	p := newPadder(w, r)
	p.write(jsonArray(reply))
}
