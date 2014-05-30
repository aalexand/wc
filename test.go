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
	p.chunk(phase1Reply)
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
// For WebChannel the format is: <code>[hostPrefix_]</code>
// For example: <code>["b"]</code>
//
// For BrowserChannel the format is: <code>[hostPrefix_,blockedPrefix_]</code>
// For example: <code>["b","chatenabled"]</code>
//
// The default is <code>[]</code> which disables both values. The default is
// acceptable for most users and should only be changed by clients with
// advanced needs.
func SetHostPrefixReply(reply string) {
	hostPrefixReply = []byte(reply)
}

// TestHandler handles WebChannel and BrowserChannel test requests. When using
// the defaults this hanlder should be installed at "/channel/test".
func TestHandler(w http.ResponseWriter, r *http.Request) {
	p := newPadder(w, r)
	if r.FormValue("MODE") == "init" {
		testPhase1(p)
	} else {
		testPhase2(p)
	}
}
