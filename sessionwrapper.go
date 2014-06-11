// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"sync"
	"time"
)

// Special cases to think about:
// * duplicate back channel
// * noop message on back channel
// * buffered proxy mode (ensure flushing all messages)
// * 'Unknown SID' error message with HTTP status 400
// * other ChannelRequest.Error error values (special cases?)
// * XSS escaping messages for client
// * backchannel handoff
// * messages delivered when no back channel exists for a session
// * client side session reconnects after server crash
// * chunk compression

// Tasks for application level:
// * sharding comet server? (moving sessions across servers?)
// * restart server without dropping back channels
// * expvar stats: # sessions, # backchannels, # pending messages, etc

var (
	mutex             sync.Mutex
	sessionWrapperMap map[string]*sessionWrapper
)

func init() {
	sessionWrapperMap = make(map[string]*sessionWrapper)
}

type sessionWrapper struct {
	Session
	si                              *SessionInfo
	reqNotifier                     chan *reqRegister
	noopTimer, longBackChannelTimer *time.Timer
	bc                              *reqRegister
	backChannelCloseNotifier        <-chan bool
	p                               *padder
	// backChannelBytes is the number of non-ACKed bytes on the back channel
	// (based upon the last AID received on a back or forward channel)
	backChannelBytes int
}

func newSessionWrapper(session Session) *sessionWrapper {
	sw := &sessionWrapper{
		Session:              session,
		si:                   &SessionInfo{-1, -1},
		reqNotifier:          make(chan *reqRegister),
		noopTimer:            time.NewTimer(30 * time.Second),
		longBackChannelTimer: time.NewTimer(4 * 60 * time.Second),
		bc:                   nil,
		backChannelCloseNotifier: nil,
		p:                nil,
		backChannelBytes: 0,
	}
	sw.noopTimer.Stop()
	sw.longBackChannelTimer.Stop()
	return sw
}

func (sw *sessionWrapper) updateBackChannelBytes() {

}
