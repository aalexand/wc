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

func TestHandler(w http.ResponseWriter, r *http.Request) {
	p, err := newPadder(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if p.t == length {
		p.t = none
	}
	p.chunk([]byte(testFirstChunk))
	time.Sleep(testDelay*time.Second)
	p.chunk([]byte(testSecondChunk))
	p.end()
}
