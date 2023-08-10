# Introduction

[中文版](./README_cn.md)

__tnet__ is an event loop networking framework that provides versatile models. The core aims are:

* Support more connections (millions of)
* Higher performance (QPS↑, latency↓)
* Less memory (use only up to ~10% memory compared with golang/net)
* Easy to use (compatible with golang/net)

Features:

* Support TCP, UDP
* Support IPv4, IPv6 
* Provide blocking/nonblocking Read/Write API
* Provide batch system calls: ReadV/WriteV 
* Support both server and client programming 
* Support Linux / Mac OS 
* Support TLS
* Support WebSocket 

## Getting Started 

For the `tnet` network library itself, it provides two classes of usage:

1. The user handler is not in the poller goroutine
2. The user handler is in the poller goroutine

As shown below:

![](./docs/pics/two_classes.png)

The `tnet` provides the first class of usage by default, and the second class of usage is opened by `tnet.WithNonBlocking(true)` (see the detailed examples in the `examples` folder).

On the basis of these two classes, the user can further differentiate into a more fine-grained mode by choosing whether to use the business goroutine pool in the handler. Specifically,

* Do not use the business goroutine pool in Class 1, which corresponds to the "classical Go/net mode". Features:

Number of goroutines = number of pollers + number of active connections. It is suitable for business scenarios where network IO and CPU processing are balanced. The disadvantage is that connection multiplexing is not supported, and concurrent processing of services cannot be supported on the same connection (IO processing part cannot be concurrently processed.)

![](./docs/pics/mode1.png)

For Class 2 where handler is in the poller goroutine, the handler function can usually be divided into the following two modes:

1. Separated IO processing and business mode
2. Combined IO processing and business mode

* Use the business goroutine pool in Class 2, which corresponds to the "separated IO processing and business mode". Features:

Number of goroutines = number of pollers + number of concurrent processing of data packets, suitable for CPU-intensive scenarios

![](./docs/pics/mode2.png)

* Do not use the business goroutine pool in Class 2, which corresponds to the " combined IO processing and business mode". Features:

Number of goroutines = number of pollers, but the use case is scarce. The classic usage is for gateway scenario where most of the logic is just forwarding requests. The processing time of each request is very low, and there will be no blocking. 

![](./docs/pics/mode3.png)

## Supported TCP Option

* `tnet.WithTCPKeepAlive` sets the time interval for keep alive. The default value is 15s. When set to 0, keep alive can be closed.

```golang
// WithTCPKeepAlive sets the tcp keep alive interval.
func WithTCPKeepAlive(keepAlive time.Duration) Option {
	return Option{func(op *options) {
		op.tcpKeepAlive = keepAlive
	}}
}
```

* `tnet.WithTCPIdleTimeout` sets the idle timeout of the connection, and the connection will be automatically disconnected when the idle time exceeds the given value.

```golang
// WithTCPIdleTimeout sets the idle timeout to close tcp connection.
func WithTCPIdleTimeout(idleTimeout time.Duration) Option {
	return Option{func(op *options) {
		op.tcpIdleTimeout = idleTimeout
	}}
}
```

* `tnet.WithOnTCPOpened` can set the operations that need to be performed when the TCP connection is just established.

```golang
// WithOnTCPOpened registers the OnTCPOpened method that is fired when connection is established.
func WithOnTCPOpened(onTCPOpened OnTCPOpened) Option {
	return Option{func(op *options) {
		op.onTCPOpened = onTCPOpened
	}}
}
```

* `tnet.WithOnTCPClosed` can set the operations that need to be performed when the TCP connection is disconnected.

```golang
// WithOnTCPClosed registers the OnTCPClosed method that is fired when tcp connection is closed.
func WithOnTCPClosed(onTCPClosed OnTCPClosed) Option {
	return Option{func(op *options) {
		op.onTCPClosed = onTCPClosed
	}}
}
```

* `tnet.WithTCPFlushWrite(true)` enables the user to complete the sending of the package directly in the current business goroutine.

```golang
// WithTCPFlushWrite sets whether use flush write for TCP
// connection or not. Default is notify.
func WithTCPFlushWrite(flush bool) Option {
	return Option{func(op *options) {
		op.flushWrite = flush
	}}
}
```

Take separated mode as an example, the procedure is like:

![](./docs/pics/mode2_flush.png)

## Supported UDP Option


* `tnet.WithOnUDPClosed` can set the operation that needs to be performed when UDP is closed.

```golang
// WithOnUDPClosed registers the OnUDPClosed method that is fired when udp connection is closed.
func WithOnUDPClosed(onUDPClosed OnUDPClosed) Option {
	return Option{func(op *options) {
		op.onUDPClosed = onUDPClosed
	}}
}
```

* `tnet.WithMaxUDPPacketSize` sets the maximum length of UDP packets.

```golang
// WithMaxUDPPacketSize sets maximal UDP packet size when receiving UDP packets.
func WithMaxUDPPacketSize(size int) Option {
	return Option{func(op *options) {
		op.maxUDPPacketSize = size
	}}
}
```

## Supported common Option


* `tnet.WithNonBlocking` can set blocking/non-blocking mode, which is also an option to control whether IO processing is in the Poller goroutine, the default is blocking mode with IO processing not in the Poller goroutine.

```golang
// WithNonBlocking set conn/packconn to nonblocking mode
func WithNonBlocking(nonblock bool) Option {
	return Option{func(op *options) {
		op.nonblocking = nonblock
	}}
}
```

## Explanation of special APIs

The `tnet.Conn` interface is extended on the basis of `net.Conn`.

* The four zero-copy APIs are as follows:


```golang
// Peek returns the next n bytes without advancing the reader. It waits until it has
// read at least n bytes or error occurs such as connection closed or read timeout.
// The bytes stop being valid at the next ReadN or Release call.
// Zero-Copy API.
Peek(n int) ([]byte, error)

// Next returns the next n bytes with advancing the reader, It waits until it has
// read at least n bytes or error occurs such as connection closed or read timeout.
// The bytes stop being valid at the next ReadN or Release call.
// Zero-Copy API.
Next(n int) ([]byte, error)

// Skip the next n bytes and advance the reader. It waits until the underlayer has at
// least n bytes or error occurs such as connection closed or read timeout.
// Zero-Copy API.
Skip(n int) error

// Release releases underlayer buffer when using Peek() and Skip() Zero-Copy APIs.
Release()
```

> 1. Peek: Read a given number of bytes, but do not move the read pointer of the underlying Linked Buffer. The returned byte slice is directly referenced from the Linked Buffer. At this time, it is required that this part of the data cannot be released before it is used up.
> 2. Skip: Move the read pointer of the underlying Linked Buffer to skip a given number of bytes, usually used with Peek.
> 3. Next: It is equivalent to calling Peek first, then calling Skip, the returned byte slice will be invalid after the call of Release.
> 4. Release: Release the read part, usually after using the byte slice. When calling the security API Read/ReadN, Release will be automatically called to release the read buffer.


* The `tnet.Conn` interface provides `Writev`, which can be used to write out multiple data blocks in turn, such as packet header and packet body, without manual data packet splicing. See `examples/tcp/classical/main.go` for details.

```golang
// Writev provides multiple data slice write in order.
Writev(p ...[]byte) (int, error)
```

* `tnet.Conn` also provides a method to sense the connection state:
```golang
// IsActive checks whether the connection is active or not.
IsActive() bool
```

* `tnet.Conn` provides `SetMetaData/GetMetadata` for storing/loading the user's private data on the connection:

```golang
// SetMetaData sets meta data. Through this method, users can bind some custom data to a connection.
SetMetaData(m interface{})
// GetMetaData gets meta data.
GetMetaData() interface{}
```

# Use cases

* tRPC-Go

tRPC-Go has integrated __tnet__.

## Implementation Details

* Overall architecture: [overview.md](./docs/overview.md)
* Goroutine Models: [models.md](./docs/models.md)
* Design and implementation of buffer: [buffer.md](./docs/buffer.md)
