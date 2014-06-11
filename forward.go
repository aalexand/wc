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

func newSessionHandler(sw *sessionWrapper, reqRequest *reqRegister) {
	debug("wc: %s forward channel (new session)", sw.SID())
	defer func() {
		reqRequest.done <- struct{}{}
	}()

	createMsg := []byte(jsonArray(
		[]interface{}{"c", sw.SID(), sm.HostPrefix(), 8},
	))
	err := sw.BackChannelAdd(createMsg)
	if err != nil {
		sm.Error(reqRequest.r, err)
		http.Error(reqRequest.w, "Unable to add create message to back channel",
			http.StatusInternalServerError)
		return
	}
	sw.backChannelBytes += len(createMsg)

	// TODO(hochhaus): how to let user code add other messages to the back
	// channel queue prior flushing down the forward channel?

	msgs, err := sw.BackChannelPeek()
	if err != nil {
		sm.Error(reqRequest.r, err)
		http.Error(reqRequest.w, "Unable to get messages",
			http.StatusInternalServerError)
		return
	}

	p := newPadder(reqRequest.w, reqRequest.r)
	p.writeMessages(msgs)
}

func fcHandler(sw *sessionWrapper, reqRequest *reqRegister) {
	debug("wc: %s forward channel", sw.SID())
	defer func() {
		reqRequest.done <- struct{}{}
	}()

	if !maybeACKBackChannel(sw, reqRequest.w, reqRequest.r, true) {
		// HTTP error codes written directly in maybeACKBackChannel().
		return
	}

	count, err := strconv.Atoi(reqRequest.r.PostFormValue("count"))
	if err != nil {
		sm.Error(reqRequest.r, err)
		http.Error(reqRequest.w, "Unable to parse count", 400)
		return
	}

	msgs := []*Message{}
	if count > 0 {
		offset, err := strconv.Atoi(reqRequest.r.PostFormValue("ofs"))
		if err != nil {
			sm.Error(reqRequest.r, err)
			http.Error(reqRequest.w, "Unable to parse ofs", 400)
			return
		}

		for i := 0; i < count; i++ {
			req := fmt.Sprintf("req%d", i)
			jsonMap := make(map[string]interface{})
			for key, value := range reqRequest.r.PostForm {
				keyParts := strings.SplitN(key, "_", 2)
				if len(keyParts) < 2 || keyParts[0] != req {
					continue
				}
				jsonMap[keyParts[1]] = value[0]
			}
			effectiveID := offset + i
			if effectiveID <= sw.si.ForwardChannelAID {
				// skip incoming messages which have already been received
				continue
			}
			msg := &Message{ID: offset + i, Body: []byte(jsonObject(jsonMap))}
			msgs = append(msgs, msg)
			debug("wc: %s new forward channel message %d %s", sw.Session.SID(),
				msg.ID, msg.Body)
		}
	}

	if len(msgs) > 0 {
		err = sw.ForwardChannel(msgs)
		if err != nil {
			sm.Error(reqRequest.r, err)
			http.Error(reqRequest.w, "Incoming message error",
				http.StatusInternalServerError)
			return
		}
	}

	reply := []interface{}{
		sw.bc != nil,
		sw.si.BackChannelAID,
		sw.backChannelBytes,
	}
	p := newPadder(reqRequest.w, reqRequest.r)
	p.write(jsonArray(reply))
}
