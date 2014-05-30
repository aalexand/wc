// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"net/http"
)

// Outside of our library
// * User adds HTTP headers
// * User implements cookie based security

// Values needed from user
// * SID generation (read from channel -- i.e. new channel created)
// * Old SID look up? (on server restart?)
// * Backchannel messages to flush down back channel (how to ack backchannel
//   messages to removal from permanent storage?)
// * Server side close/stop active session (eg: log a user out due to account
//   deletion, cookie timeout, etc)

// Values needed to pass to user
// * backchannel close (http.CloseNotifier)
// * forward channel messages (how to ack delivered? wait until CB done?
//   2nd channel? how to ask user if SID and CID match?)
// * client JS disconnect/terminate
// * session timeout (including sessions from before server restart)
// * error logging

// Special cases:
// * duplicate back channel
// * noop message on back channel
// * buffered proxy mode (ensure flushing all messages)
// * 'Unknown SID' error message with HTTP status 400
// * other ChannelRequest.Error error values (special cases?)
// * XSS escaping messages for client
// * sharding comet server? (moving sessions across servers?)
// * backchannel handoff
// * messages delivered when no back channel exists for a session
// * OSID, OAID session restarts
// * client side session reconnects after server crash
// * restart server without dropping back channels

// BindHandler handles forward and backward channel HTTP requests. When using
// the defaults this handler should be installed at "/channel" (WebChannel) or
// "/channel/bind" (BrowserChannel).
func BindHandler(w http.ResponseWriter, r *http.Request) {
	// back channel
	// terminate
	// forward channel
}
