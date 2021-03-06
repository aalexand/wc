wc
============

**This library was used to build [dropinchat.com](https://news.ycombinator.com/item?id=8129397) for YC Hacks.**

A [Go](http://golang.org/) web server compatible with
[closure-library](https://github.com/google/closure-library)'s
[goog.net.WebChannel](https://github.com/google/closure-library/blob/master/closure/goog/labs/net/webchannel.js).
A WebChannel represents a logical bi-directional communication channel between
client and server. By exposing a generic communication interface which can be
implemented over a variety of transports (eg: BrowserChannel, WebSockets,
WebRTC, etc) WebChannel provides additional flexibility over programming
directly on top of WebSockets.

The client-side portion of WebChannel is open sourced (APLv2) as part of
closure-library. Unfortunately, [Google has not released the server-side
portion](http://books.google.com/books?id=p7uyWPcVGZsC&pg=PA179) of the
code required to use WebChannel meaningfully. The wc package provides an
open source (BSD) licensed golang server-side library to fill this missing gap.

See the [wcchat](https://github.com/samegoal/wcchat) package for an example
application.

WebChannel
----------

From [goog.net.WebChannel](https://github.com/google/closure-library/blob/master/closure/goog/labs/net/webchannel.js#L16):

> Similar to HTML5 WebSocket and Closure BrowserChannel, WebChannel
> offers an abstraction for point-to-point socket-like communication between
> a browser client and a remote origin.
>
> WebChannels are created via <code>WebChannel</code>. Multiple WebChannels
> may be multiplexed over the same WebChannelTransport, which represents
> the underlying physical connectivity over standard wire protocols
> such as HTTP and SPDY.
>
> A WebChannels in turn represents a logical communication channel between
> the client and server end point. A WebChannel remains open for
> as long as the client or server end-point allows.
>
> Messages may be delivered in-order or out-of-order, reliably or unreliably
> over the same WebChannel. Message delivery guarantees of a WebChannel is
> to be specified by the application code; and the choice of the
> underlying wire protocols is completely transparent to the API users.
>
> Client-to-client messaging via WebRTC based transport may also be support
> via the same WebChannel API in future.

At the time of this writing (5/2014) the only WebChannel transport included
in closure-library is BrowserChannel. As additional transports are added
wc intends to add support for them as well.

BrowserChannel
--------------

From [goog.net.BrowserChannel](https://github.com/google/closure-library/blob/master/closure/goog/net/browserchannel.js#L16):

> A BrowserChannel simulates a bidirectional socket over HTTP. It is the
> basis of the Gmail Chat IM connections to the server.

BrowserChannel works on all major browsers (including IE5.5+) using a variety
of technologies including forever iframes (IE < 10) and XHR Streaming (IE10+
and non-IE).

Client Usage (JavaScript)
-------------------------

To connect from the client:

  * [goog.net.WebChannel.Options](https://github.com/google/closure-library/blob/master/closure/goog/labs/net/webchannel.js#L63)
  * Implement a BrowserChannel.Handler subclass
  * Instantiate a BrowserChannel and connect()
  * call channel.sendMap() to send data

```javascript
goog.require('goog.net.createWebChannelTransport');
goog.require('goog.net.WebChannel');

/**
 * @type {!goog.net.WebChannelTransport}
 */
var channelTransport = goog.net.createWebChannelTransport();

/**
 * @type {!goog.net.WebChannel.Options}
 */
var options = { supportsCrossDomainXhr: true };

/**
 * @type {!goog.net.WebChannel}
 */
var channel = channelTransport.createWebChannel('/channel', options);


/**
 * Browser channel handler.
 * @constructor
 * @extends {goog.net.BrowserChannel.Handler}
 */
demo.ChannelHandler = function() {};
goog.inherits(demo.ChannelHandler, goog.net.BrowserChannel.Handler);

/** @inheritDoc */
demo.ChannelHandler.prototype.channelHandleArray = function(browserChannel, array) {
  ...
};

var handler = new demo.ChannelHandler();
var channelDebug = new goog.net.ChannelDebug();
var channel = new goog.net.BrowserChannel('8', ['<host prefix>', '<blocked prefix>']);
channel.setSupportsCrossDomainXhrs(true);
channel.setHandler(handler);
channel.setChannelDebug(channelDebug);
channel.connect('channel/test', 'channel/bind', {});

channel.sendMap(...);

channel.disconnect();
```

Server Usage (Go)
-----------------

TODO(ahochhaus): Document

```
go get gopkg.in/samegoal/wc.v0
```

[Docs](http://godoc.org/gopkg.in/samegoal/wc.v0)

[Demo Chat Application](https://github.com/samegoal/wcchat)

Alternate Implementations
-------------------------

  * C++: [libevent-browserchannel-server](https://code.google.com/p/libevent-browserchannel-server/);
    [protocol documentation](https://code.google.com/p/libevent-browserchannel-server/wiki/BrowserChannelProtocol)
  * Node.JS: [node-browserchannel](https://github.com/josephg/node-browserchannel/)
  * Go (golang): [go-browserchannel](https://github.com/MathieuTurcotte/go-browserchannel)
  * Clojure [clj-browserchannel](https://github.com/thegeez/clj-browserchannel)

Thanks
------

  * mdavids, upstream author of BrowserChannel, for [all](https://groups.google.com/forum/?fromgroups#!topic/closure-library-discuss/0xy-2yPyUII)
  [of](https://groups.google.com/forum/?fromgroups#!topic/closure-library-discuss/b4q1JfrBkjI)
  [the](https://groups.google.com/forum/#!msg/closure-library-discuss/F1mtsUK1NIM/GsrAU7KfS8cJ)
  [help](https://groups.google.com/forum/?fromgroups#!topic/closure-library-discuss/BRs3JSwm3Dc)
  he provided.
