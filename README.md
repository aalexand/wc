gowebchannel
============

[Full-duplex](http://en.wikipedia.org/wiki/Full-duplex#Full-duplex) web
server compatible with [goog.net.WebChannel](https://code.google.com/p/closure-library/source/browse/closure/goog/labs/net/webchannel.js) (from [closure-library](https://code.google.com/p/closure-library/))
and written in [Go](http://golang.org/).

The client-side portion of WebChannel is open sourced (APLv2) as part of
closure-library. Unfortunately, [Google has not released the server-side
portion](http://books.google.com/books?id=p7uyWPcVGZsC&pg=PA179) of the
code required to use WebChannel meaningfully. The gowebchannel package
provides an open source (BSD) licensed server-side library to fill this
missing gap.

WebChannel
----------

From the [goog.net.WebChannel file overview](https://code.google.com/p/closure-library/source/browse/closure/goog/labs/net/webchannel.js#16):

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

At the time of this writing (5/2013) the only WebChannel transport included
in closure-library is BrowserChannel. As additional transports are added
gowebchannel intends to add support for them as well.

BrowserChannel
--------------

From the [goog.net.BrowserChannel](http://docs.closure-library.googlecode.com/git/class_goog_net_BrowserChannel.html)
[file overview](https://code.google.com/p/closure-library/source/browse/closure/goog/net/browserchannel.js#16):

> A BrowserChannel simulates a bidirectional socket over HTTP. It is the
> basis of the Gmail Chat IM connections to the server.

BrowserChannel works on all major browsers (including IE6) using a variety of
technologies including forever iframes (IE < 10) and XHR Streaming (IE10+ and
non-IE). 

Client Usage (JavaScript)
-------------------------

To connect from the client:

  * Implement a BrowserChannel.Handler subclass
  * Instantiate a BrowserChannel and connect()
  * call channel.sendMap() to send data


```javascript
goog.require('goog.net.BrowserChannel');
goog.require('goog.net.BrowserChannel.Handler');

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

Alternate Implementations
-------------------------

  * C++: [libevent-browserchannel-server](https://code.google.com/p/libevent-browserchannel-server/);
    [protocol documentation](https://code.google.com/p/libevent-browserchannel-server/wiki/BrowserChannelProtocol)
  * Java: [Google Web Toolkit](https://code.google.com/p/google-web-toolkit/source/browse/trunk/dev/core/src/com/google/gwt/dev/shell/BrowserChannelServer.java);
    [Design](https://code.google.com/p/google-web-toolkit/wiki/DesignOOPHM)
  * JavaScript: [node-browserchannel](https://github.com/josephg/node-browserchannel/)
  * Go: [go-browserchannel](https://github.com/MathieuTurcotte/go-browserchannel)

Thanks
------

  * mdavids, upstream author of BrowserChannel, for [all](https://groups.google.com/forum/?fromgroups#!topic/closure-library-discuss/0xy-2yPyUII)
  [of](https://groups.google.com/forum/?fromgroups#!topic/closure-library-discuss/b4q1JfrBkjI)
  [the](https://groups.google.com/forum/#!msg/closure-library-discuss/F1mtsUK1NIM/GsrAU7KfS8cJ)
  [help](https://groups.google.com/forum/?fromgroups#!topic/closure-library-discuss/BRs3JSwm3Dc)
  he provided.
