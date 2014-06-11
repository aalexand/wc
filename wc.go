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
	"log"
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
	ServerTerminate SessionActivity = 0
)

// TerminationReason denotes the reason a session is being terminated.
type TerminationReason int

const (
	// ClientTerminateRequest denotes the client explicitly requesting the
	// SID be terminated.
	ClientTerminateRequest TerminationReason = iota

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

// NewMessage creates a new Message struct.
func NewMessage(ID int, Body []byte) *Message {
	return &Message{ID, Body}
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

	// DataNotifier() provides a channel for application code to pass byte counts
	// each time new BackChannel data arrives. Writes to this session will not
	// block so it is safe to write to this channel even from inside the
	// session's go routine (for example from the ForwardChannel() callback).
	DataNotifier() chan int

	// BackChannelNewSessionMessages allows insertion of messages into the back
	// channel queue of a new session. Do not add messages from NewSession().
	BackChannelNewSessionMessages() error

	// BackChannelPeek returns a slice of all pending backchannel Messages.
	BackChannelPeek() ([]*Message, error)

	// BackChannelClose notifies that the current backchannel has closed. Most
	// applications do not need this and can rely on the default implementation.
	BackChannelClose()

	// BackChannelOpen notifies that a new backchannel has opened. Most
	// applications do not need this and can rely on the default implementation.
	BackChannelOpen()

	// BackChannelAckThrough notifies that all messages up to an including ID
	// have been successfully delivered to the client. These messages should now
	// be garbage collected.
	BackChannelACKThrough(ID int) error

	// BackChannelAdd adds the message to into the pending backchannel queue.
	// This functionality is necessary to support WebChannel internal messages
	// such as "create", "noop" and "stop". These internal messages are passed to
	// the application layer to allow for a shared message numbering scheme on
	// the wire and application level.
	BackChannelAdd(messageBody []byte) error

	// ForwardChannel passes the set of messages delivered from the client to the
	// application for persistent storage (eg: being added to a "MessageQueue"
	// for later processing) or processed synchronously prior to returning from
	// ForwardChannel. At the point that ForwardChannel() returns the messages
	// will be ACKed to the client and not attempted to be redelivered.
	// Therefore, it is vital that a non-nil error be returned (such that
	// no ACK is sent to the client) if the messages could not be added to
	// storage for later processing (or processed synchronously).
	ForwardChannel(msgs []*Message) error
}

// SessionInfo tracks the state related to which messages have been processed
// on the forward and backward channels.
type SessionInfo struct {
	// BackChannelAID should be set to the lowest ID in the back channel message
	// queue when computing the return value for LookupSession().
	BackChannelAID int

	// ForwardChannelAID should be set to the largest previously received forward
	// channel ID when computing the return value for LookupSession().
	ForwardChannelAID int
}

// DefaultSession provides a partial implementation of the Session interface.
// Callers must implement at least BackChannel(), AckBackChannelThrough(),
// BackChannelAdd() and ForwardChannel().
type DefaultSession struct {
	SessionID    string
	notifier     chan SessionActivity
	dataNotifier chan int
}

// NewDefaultSession initializes a DefaultSession object with the specified ID.
func NewDefaultSession(sid string) *DefaultSession {
	return &DefaultSession{sid, make(chan SessionActivity), make(chan int)}
}

// SID return the SessionID field.
func (s *DefaultSession) SID() string {
	return s.SessionID
}

// Notifier returns the DefaultSession notifier chan.
func (s *DefaultSession) Notifier() chan SessionActivity {
	return s.notifier
}

// DataNotifier returns the DefaultSession notifier chan.
func (s *DefaultSession) DataNotifier() chan int {
	return s.dataNotifier
}

// BackChannelNewSessionMessages provides a noop implementation.
func (s *DefaultSession) BackChannelNewSessionMessages() error {
	return nil
}

// BackChannelClose provides a noop implementation.
func (s *DefaultSession) BackChannelClose() {
}

// BackChannelOpen provides a noop implementation.
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
	LookupSession(sid string) (Session, *SessionInfo, error)

	// NewSession creates a new WebChannel session. The returned Session object
	// must have SID() populated. Additionally, session persistent state should
	// be created as necessary. Do not add messages to the back channel from
	// this function, instead see BackChannelNewSessionMessages().
	NewSession(r *http.Request) (Session, error)

	// TerminatedSession notifies that the Session has been terminated (either
	// by the client or server).
	TerminatedSession(sid string, reason TerminationReason) error

	// Error logs internal failure conditions to application level code.
	Error(r *http.Request, err error)

	// Debug logs internal wc debugging messages. This is most useful to
	// developers of wc.
	Debug(string)

	// HostPrefix is used on IE < 10 to circumvent same host connection limits.
	//
	// On the client, hostPrefix_ values will be passed to correctHostPrefix()
	// prior to use.
	//
	// WebChannel: https://github.com/google/closure-library/blob/master/closure/goog/labs/net/webchannel/webchannelbase.js#L151
	// BrowserChannel: https://github.com/google/closure-library/blob/master/closure/goog/net/browserchannel.js#L235
	//
	// The default, disabling the host prefix, is acceptable for most users. This
	// library does not support BlockedPrefix (used by BrowserChannel only).
	HostPrefix() string
}

// DefaultSessionManager provides a partial implementation of the
// SessionManager interface. Callers must implement at least Authenticated(),
// NewSession() and TerminatedSession().
type DefaultSessionManager struct {
}

// LookupSession provides a noop implementation. All sessions requested are
// returned as ErrUnknownSID which is suitable for applications which do not
// persist session information across server server restarts.
func (sm *DefaultSessionManager) LookupSession(sid string) (
	Session,
	*SessionInfo,
	error,
) {
	return nil, nil, ErrUnknownSID
}

// Error logs to stderr.
func (sm *DefaultSessionManager) Error(r *http.Request, err error) {
	log.Print(err.Error())
	panic("error")
}

// Debug discards debugging messages.
func (sm *DefaultSessionManager) Debug(debug string) {
	log.Print(debug)
}

// HostPrefix provides an empty host prefix.
func (sm *DefaultSessionManager) HostPrefix() string {
	return ""
}

// SetSessionManager allows the calling code to inject a custom application
// specific SessionManager implementation.
func SetSessionManager(sessionMgr SessionManager) {
	if sm != nil {
		panic("SessionManager already specified")
	}
	sm = sessionMgr
}
