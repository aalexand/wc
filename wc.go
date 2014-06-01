// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wc implements a pure-Go web server capable of bi-directional
// communication between a Go (golang) net.http server and a in-browser client.
// Both goog.net.WebChannel and goog.net.BrowserChannel from closure-library
// are supported client-side.
package wc

import (
	"net/http"
)

// SessionActivity sends notifications from application level code to the wc
// library.
type SessionActivity int

const (
	// BackChannelActivity notifies wc that the given session has pending
	// messages which should be flushed to the next avaliable backchannel.
	BackChannelActivity SessionActivity = iota
	// ServerTerminate notifies wc the the given session should be terminated.
	// A common use case is automatically logging a user out.
	ServerTerminate
)

// Message describes a single forward or backchannel message.
type Message struct {
	ID   int
	Body []byte
}

// Session specifies the interface for the calling application to interact
// with an individual WebChannel session. This is used to both modify the
// Session and receive events from it.
type Session interface {
	SID() string
	Notifier() chan SessionActivity
	BackChannel() ([]Message, error)
	AckBackChannelThrough(ID int) error
	ForwardChannel(Message) error
	ClientTerminateSession() error
}

// SessionManager specifies the interface for the calling application to
// interact with newly created and resumed sessions.
type SessionManager interface {
	Authenticated(sid string, r *http.Request) bool
	LookupSession(sid string) (Session, error)
	NewSession(r *http.Request) (Session, error)
}

// SetSessionManager allows th calling for to inject a custom application
// specific implementation.
func SetSessionManager() {
}
