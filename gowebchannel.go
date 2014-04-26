// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gowebchannel implements a pure Go full-duplex web server compatible
// with goog.net.WebChannel (from closure-library).
//
// The client-side portion of WebChannel is open sourced (APLv2) as part of
// closure-library. Unfortunately, Google has not released the server-side
// portion of the code required to use WebChannel meaningfully. The
// gowebchannel package provides an open source (BSD) licensed server-side
// library to fill this missing gap.
package gowebchannel
