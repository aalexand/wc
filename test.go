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

// TODO(ahochhaus): Do not write using chunk() interface
func testPhase1(p *padder) {
	p.t = none
	p.chunk(jsonArray(sm.HostPrefix()))
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
