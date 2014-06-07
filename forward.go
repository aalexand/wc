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

func newSessionHandler(
	sw *sessionWrapper,
	w http.ResponseWriter,
	r *http.Request,
) {
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
		msgs = append(msgs, peekMsgs...)
	}

	p := newPadder(w, r)
	p.writeMessages(msgs)
}

func fcHandler(
	sw *sessionWrapper,
	hasBackChannel bool,
	w http.ResponseWriter,
	r *http.Request,
) {
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
				jsonMap[keyParts[1]] = value[0]
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
		hasBackChannel,
		sw.si.BackChannelAID,
		sw.si.BachChannelBytes,
	}
	p := newPadder(w, r)
	p.write(jsonArray(reply))
}
