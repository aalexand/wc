// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wc implements a pure-Go web server capable of bi-directional
// communication between a Go (golang) net.http server and a in-browser client.
// Both goog.net.WebChannel and goog.net.BrowserChannel from closure-library
// are supported client-side.
package wc

import (
	"errors"
	"net/http"
)

// TODO(hochhaus): OSID, OAID session restarts

var (
	// ErrUnknownSID is the error to be returned when the requested SID is not
	// known to the server.
	ErrUnknownSID = errors.New("wc: Unknown SID")

	sm SessionManager
)

// SessionActivity sends notifications from application level code to the wc
// library.
type SessionActivity int

const (
	// BackChannelActivity notifies wc that the given session has pending
	// messages which should be flushed to the next available backchannel.
	BackChannelActivity SessionActivity = iota

	// ServerTerminate notifies wc the the given session should be terminated.
	// Common use cases include Session timeouts, automatically logging a user
	// out of the web application, or terminating a session out from under an
	// active user when their account is deleted.
	//
	// It is the responsibility of the application level code to send
	// ServerTerminate notifications when active sessions should be timed out.
	// Failure to do so will cause leaking Session objects and the goroutines
	// processing those sessions. Additionally, it is the responsibility of the
	// application level code to cleanup old sessions which are no longer used
	// but might be retained in persistent storage (such as a message queue).
	// Such session "leakage" can happen, for example, when the server crashes
	// and a session which was previously being processed (and would have been
	// timed-out) it not used again after the application restarts.
	ServerTerminate
)

// TerminationResaon denotes the reason a session is being terminated.
type TerminationResaon int

const (
	// ClientTerminateRequest denotes the client explicitly requesting the
	// SID be terminated.
	ClientTerminateRequest TerminationResaon = iota

	// ServerTerminateRequest denotes application code on the server requesting
	// the termination of the SID by sending a ServerTerminate event to the
	// Session Notifier().
	ServerTerminateRequest
)

// Message describes a single forward or backchannel message.
type Message struct {
	ID   int
	Body []byte
}

// Session specifies the interface for the calling application to interact
// with an individual WebChannel session. This is used to both modify the
// Session and receive events from it. Only a single method will be invoked
// per session at a time.
type Session interface {
	SID() string

	// Notifier provides the channel for application code to pass SessionActivity
	// events for processing by WebChannel.
	Notifier() chan SessionActivity

	// BackChannel returns a slice of all pending Messages to be sent down the
	// backchannel.
	BackChannel() ([]Message, error)

	// BackChannelClose notifies that the current backchannel has closed. Most
	// applications do not need this and can rely on the default implementation.
	BackChannelClose()

	// BackChannelOpen notifies that a new backchannel has opened. Most
	// applications do not need this and can rely on the default implementation.
	BackChannelOpen()

	// AckBackChannelThrough notifies that all messages up to an including ID
	// have been successfully delivered to the client. These messages should now
	// be garbage collected.
	AckBackChannelThrough(ID int) error

	// ForwardChannel passes the set of messages delivered from the client to the
	// application for persistent storage (eg: being added to a "MessageQueue"
	// for later processing) or processed synchronously prior to returning from
	// ForwardChannel. At the point that ForwardChannel() returns the messages
	// will be ACKed to the client and not attempted to be redelivered.
	// Therefore, it is vital that a non-nil error be returned (such that
	// no ACK is sent to the client) if the messages could not be added to
	// storage for later processing (or processed synchronously).
	ForwardChannel(msgs []Message) error

	// Terminated notifies that the Session has been terminated (either by the
	// client or server).
	TerminateSession(reason TerminationResaon) error
}

// DefaultSession provides a partial implementation of the Session interface.
// Callers must implement at least BackChannel(), AckBackChannelThrough(),
// ForwardChannel() and TerminateSession().
type DefaultSession struct {
	SessionID string
	notifier  chan SessionActivity
}

// SID return the SessionID field.
func (s *DefaultSession) SID() string {
	return s.SessionID
}

// Notifier returns the DefaultSession notifier chan.
func (s *DefaultSession) Notifier() chan SessionActivity {
	return s.notifier
}

// BackChannelClose provide a noop implementation.
func (s *DefaultSession) BackChannelClose() {
}

// BackChannelOpen provide a noop implementation.
func (s *DefaultSession) BackChannelOpen() {
}

// SessionManager specifies the interface for the calling application to
// interact with newly created and resumed sessions. Multiple methods can be
// invoked concurrently from different goroutines (make sure to protect your
// state accordingly).
//
// Some actions (eg: adding common HTTP headers, HTTP request logging, etc)
// should be performed by the application code outside of the wc
// Session{,Manager} interfaces. For an example of doing so, see the wcchat
// demo application.
type SessionManager interface {
	// Authenticated verifies that the sid and request pair are correctly
	// associated and have access to the user application. If Authenticated()
	// returns true back/forward channel messages will be sent/received.
	//
	// This check only determines if the given HTTP request should be able to
	// send and receive messages for the specified sid. Additional application
	// level security is required for almost all applications.
	Authenticated(sid string, r *http.Request) bool

	// Lookup a previous session which is unknown to the server at this time.
	// This is useful to clients which store persistent session information
	// which survives across server restarts. When a requested SID cannot be
	// found, return ErrUnknownSID.
	LookupSession(sid string) (*Session, error)

	// NewSession creates a new WebChannel session. The returned *Session object
	// must have SID() populated. Additionally, session persistent state should
	// be created as necessary.
	NewSession(r *http.Request) (*Session, error)

	// Error logs internal failure conditions to application level code.
	Error(sid string, r *http.Request, err error)
}

// SetSessionManager allows the calling code to inject a custom application
// specific SessionManager implementation.
func SetSessionManager(sessionMgr SessionManager) {
	if sm != nil {
		panic("SessionManager already specified")
	}
	sm = sessionMgr
}
