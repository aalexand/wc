// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"net/http"
	"time"
)

const (
	testFirstChunk  = "11111"
	testSecondChunk = "2"
	testDelay       = 2
)

var (
	hostPrefixReply = []byte("[]")
)

func testPhase1(p *padder) {
	p.t = none
	p.chunk(hostPrefixReply)
	p.end()
}

func testPhase2(p *padder) {
	if p.t == length {
		p.t = none
	}
	err := p.chunk([]byte(testFirstChunk))
	if err != nil {
		return
	}
	time.Sleep(testDelay * time.Second)
	p.chunk([]byte(testSecondChunk))
	p.end()
}

// SetHostPrefixReply initializes the response for phase 1 of the network test.
// The host prefix is only used on IE < 10 to circumvent same host connection
// limits. The blocked prefix is only supported by BrowserChannel (not
// WebChannel) to allow network admins to block chat functionality.
//
// http://www.google.com/support/chat/bin/answer.py?answer=161980
//
// SetHostPrefixReply must be called prior to listening for requests.
//
// For WebChannel the format is: [hostPrefix_]
// For example: ["b"]
// https://github.com/google/closure-library/blob/master/closure/goog/labs/net/webchannel/webchannelbase.js#L151
//
// For BrowserChannel the format is: [hostPrefix_,blockedPrefix_]
// For example: ["b","chatenabled"]
// https://github.com/google/closure-library/blob/master/closure/goog/net/browserchannel.js#L235
// https://github.com/google/closure-library/blob/master/closure/goog/net/browsertestchannel.js#L165
//
// The default is "[]" which disables both host and blocked prefixes. The
// default is acceptable for most users.
func SetHostPrefixReply(reply string) {
	hostPrefixReply = []byte(reply)
}

// TestHandler handles WebChannel and BrowserChannel test requests. When using
// the defaults this hanlder should be installed at "/channel/test".
func TestHandler(w http.ResponseWriter, r *http.Request) {
	if sm == nil {
		panic("No SessionManager provided")
	}
	p := newPadder(w, r)
	switch r.FormValue("MODE") {
	case "init":
		testPhase1(p)
	default:
		testPhase2(p)
	}
}
