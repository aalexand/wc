// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wc implements a pure-Go web server capable of bi-directional
// communcation between client and server using goog.net.WebChannel (from
// closure-library).
package wc

import (
	"net/http"
)

func BindHandler(w http.ResponseWriter, r *http.Request) {
}
