//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 THL A29 Limited, a Tencent company.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

package websocket

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/pkg/errors"
	"trpc.group/trpc-go/tnet"
	"trpc.group/trpc-go/tnet/tls"
)

// conn implements Conn interface.
type conn struct {
	// mu enables concurrent writes of websocket messages.
	// Although tnet.Conn.Write has its own lock internally, wsutil.WriteMessage needs
	// to call tnet.Conn.Write twice: first to write the header and then to write the body,
	// leading to the need for an additional lock here to ensure that the packet header and
	// the packet body are written to the connection as a whole.
	// Note: This lock does not guarantee concurrent safety when reading message.
	mu          sync.Mutex
	raw         rawConnection
	metaData    any
	role        ws.State    // ws.StateServerSide or ws.StateClientSide.
	reader      io.Reader   // connection-wise message type reader for Read.
	messageType MessageType // connection-wise message type for Read/Write.
	subprotocol string      // the subprotocol selected during handshake.
	pingHandler func(c Conn, data []byte) error
	pongHandler func(c Conn, data []byte) error
}

// rawConnection provides an interface that raw connection inside *websocket.conn
// needs to implement.
type rawConnection interface {
	net.Conn
	// Writev provides multiple data slice write in order.
	Writev(p ...[]byte) (int, error)
	// SetIdleTimeout sets the idle timeout to close connection.
	SetIdleTimeout(d time.Duration) error
	// SetMetaData sets meta data. Through this method, users can bind some custom data to a connection.
	SetMetaData(any)
	// GetMetaData gets meta data.
	GetMetaData() any
}

// rawConn wraps tls.Conn to provide a pseudo Writev implementation.
type rawConn struct {
	tls.Conn
}

// Writev implements websocket.RawConn interface.
func (c *rawConn) Writev(p ...[]byte) (int, error) {
	var num int
	for i := range p {
		n, err := c.Write(p[i])
		if err != nil {
			return num, err
		}
		num += n
	}
	return num, nil
}

// Read implements net.Conn. It is used only when message type of this connection
// is set. The connection-wise message type is set by websocket.WithMessageType
// when server is created and cannot be changed thereafter.
// Errors will be returned if message type is not set or the read message type is
// not equal to the set message type.
//
// Not Concurrent safe.
// Do not use this API in multiple goroutines.
func (c *conn) Read(buf []byte) (int, error) {
	if c.messageType != Text && c.messageType != Binary {
		return 0, errors.New("message type is neither Text nor binary for this connection, cannot use Read")
	}
	for {
		if c.reader == nil {
			var (
				err error
				tp  MessageType
			)
			tp, c.reader, err = c.NextMessageReader()
			if tp != c.messageType {
				io.Copy(io.Discard, c.reader) // Discard the mismatch message.
				return 0, fmt.Errorf("inconsistent message type from Read: %s, want %s", tp, c.messageType)
			}
			if err != nil {
				return 0, err
			}
		}
		n, err := c.reader.Read(buf)
		if err == io.EOF {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

// ReadMessage reads a complete text or binary data message.
// The returned DataType specifies that it is text or binary.
// The returned type can only be text or binary, because control
// frame are automatically handled by control handlers.
//
// Not Concurrent safe.
// Do not use this API in multiple goroutines.
func (c *conn) ReadMessage() (MessageType, []byte, error) {
	tp, rd, err := c.NextMessageReader()
	if err != nil {
		return tp, nil, err
	}
	bts, err := ioutil.ReadAll(rd)
	return tp, bts, err
}

// NextMessageReader returns a reader to read the next message.
//
// If you want to use websocket as a plain byte stream protocol,
// try to wrap it with a customized connection and re-create a
// new reader in a loop to retrieve the payload bytes inside
// multiple messages:
//
//	type customizedConn struct {
//		c      *websocket.conn
//		reader io.Reader
//	}
//
//	Read implements io.Reader, treats websocket protocol as a plain
//	binary data stream protocol. The returned data type must be of type binary.
//	Payloads from multiple messages can be read to fill the given buffer p.
//	func (c *customizedConn) Read(p []byte) (int, error) {
//		var (
//			tp  websocket.MessageType
//			err error
//		)
//		tp, c.reader, err = c.c.NextMessageReader()
//		if err != nil {
//			return 0, err
//		}
//		if tp != Binary {
//			return 0, errors.New("must read binary data type from Read")
//		}
//		for {
//			if c.reader == nil {
//				var err error
//				_, c.reader, err = c.c.NextMessageReader()
//				if err != nil {
//					return 0, err
//				}
//			}
//			n, err := c.reader.Read(p)
//			if err == io.EOF {
//				c.reader = nil
//				if n > 0 {
//					return n, nil
//				}
//				continue
//			}
//			return n, err
//		}
//	}
//
// Not Concurrent safe.
// Do not use this API in multiple goroutines.
func (c *conn) NextMessageReader() (MessageType, io.Reader, error) {
	controlHandler := c.newControlFrameHandler()
	rd := c.newReader(controlHandler)
	for {
		hdr, err := rd.NextFrame()
		if err != nil {
			return toMessageType[hdr.OpCode], nil, err
		}
		// Process control frames and get a next frame.
		if hdr.OpCode.IsControl() {
			if err := controlHandler(hdr, rd); err != nil {
				return toMessageType[hdr.OpCode], nil, err
			}
			continue
		}
		return toMessageType[hdr.OpCode], rd, err
	}
}

// Write implements net.Conn. Connection-wise message type should be set using
// func websocket.WithMessageType in order to use this API. It will write a message
// of connection-wise message type.
// Error will be returned if message type is not set.
//
// Concurrent safe.
// You can use this API in multiple goroutines.
func (c *conn) Write(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.messageType != Text && c.messageType != Binary {
		return 0, errors.New("message type is neither Text nor Binary for this connection, cannot use Write")
	}
	if err := c.writeMessage(c.messageType, buf); err != nil {
		return 0, err
	}
	return len(buf), nil
}

// WriteMessage writes a message in a single frame.
//
// Concurrent safe.
// You can use this API in multiple goroutines.
func (c *conn) WriteMessage(tp MessageType, buf []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeMessage(tp, buf)
}

// WritevMessage writes multiple messages in a single frame.
// Note that client side needs to mask the data to form payload,
// therefore writev does not actually works with client side writing.
//
// Concurrent safe.
// You can use this API in multiple goroutines.
func (c *conn) WritevMessage(tp MessageType, p ...[]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.role.ClientSide() {
		return c.clientWritev(tp, p...)
	}
	return c.serverWritev(tp, p...)
}

func (c *conn) serverWritev(tp MessageType, p ...[]byte) error {
	var length int64
	for i := range p {
		length += int64(len(p[i]))
	}
	h := ws.Header{
		Fin:    true,
		OpCode: toOpCode[tp],
		Length: length,
	}
	if err := ws.WriteHeader(c.raw, h); err != nil {
		return err
	}
	_, err := c.raw.Writev(p...)
	return err
}

// clientWritev does not use writev, because masking needs to be done
// for client side.
func (c *conn) clientWritev(tp MessageType, p ...[]byte) error {
	var payload []byte
	for i := range p {
		payload = append(payload, p[i]...)
	}
	return c.writeMessage(tp, payload)
}

func (c *conn) writeMessage(tp MessageType, buf []byte) error {
	return wsutil.WriteMessage(c.raw, c.role, toOpCode[tp], buf)
}

// NextMessageWriter return a writer to write the next message.
// A finished message write should end with writer.Close().
//
// Not Concurrent safe.
// Do not use this API in multiple goroutines.
func (c *conn) NextMessageWriter(tp MessageType) (io.WriteCloser, error) {
	return &writeCloser{wsutil.NewWriter(c.raw, c.role, toOpCode[tp])}, nil
}

type writeCloser struct {
	*wsutil.Writer
}

// Close implements io.Closer.
func (w *writeCloser) Close() error {
	return w.Flush()
}

// SetMetaData sets meta data.
func (c *conn) SetMetaData(m any) {
	c.metaData = m
}

// GetMetaData gets meta data.
func (c *conn) GetMetaData() any {
	return c.metaData
}

// Close closes the websocket connection with error code and reason.
func (c *conn) Close() error {
	// Not necessary to call the options.onClose,
	// since options.onClose has already been registered
	// into tnet's onClose.
	return c.raw.Close()
}

// Subprotocol returns the negotiated protocol for the connection.
func (c *conn) Subprotocol() string {
	return c.subprotocol
}

// SetPingHandler sets customized Ping frame handler.
func (c *conn) SetPingHandler(handler func(Conn, []byte) error) {
	c.pingHandler = handler
}

// SetPongHandler sets customized Pong frame handler.
func (c *conn) SetPongHandler(handler func(Conn, []byte) error) {
	c.pongHandler = handler
}

func (c *conn) newReader(handler wsutil.FrameHandlerFunc) *wsutil.Reader {
	return &wsutil.Reader{
		Source:         c.raw,
		State:          c.role,
		CheckUTF8:      true,
		OnIntermediate: handler,
	}
}

func (c *conn) newControlFrameHandler() wsutil.FrameHandlerFunc {
	return func(h ws.Header, r io.Reader) error {
		return newControlHandler(c, r).handle(h)
	}
}

// SetIdleTimeout sets connection level idle timeout.
func (c *conn) SetIdleTimeout(d time.Duration) error {
	return c.raw.SetIdleTimeout(d)
}

// LocalAddr returns the local network address, if known.
func (c *conn) LocalAddr() net.Addr {
	return c.raw.LocalAddr()
}

// RemoteAddr returns the remote network address, if known.
func (c *conn) RemoteAddr() net.Addr {
	return c.raw.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c *conn) SetDeadline(t time.Time) error {
	return c.raw.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *conn) SetReadDeadline(t time.Time) error {
	return c.raw.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *conn) SetWriteDeadline(t time.Time) error {
	return c.raw.SetWriteDeadline(t)
}

// SetOnRequest sets request handler for websocket connection.
// Typically used by websocket client.
func (c *conn) SetOnRequest(handler Handler) error {
	if tc, ok := c.raw.(tnet.Conn); ok {
		return tc.SetOnRequest(func(conn tnet.Conn) error {
			o := defaultServerOptions
			return handleWithOptions(conn, handler, &o)
		})
	}
	return errors.New("websocket.conn is expected to have raw to be tnet.Conn in SetOnRequest")
}

// SetOnClosed set on closed function for websocket connection.
func (c *conn) SetOnClosed(oc OnClosed) error {
	if tc, ok := c.raw.(tnet.Conn); ok {
		return tc.SetOnClosed(onClosed(oc))
	}
	return errors.New("websocket.conn is expected to have raw to be tnet.Conn in SetOnClosed")
}

var toMessageType = map[ws.OpCode]MessageType{
	ws.OpText:   Text,
	ws.OpBinary: Binary,
	ws.OpPing:   Ping,
	ws.OpPong:   Pong,
	ws.OpClose:  Close,
}

var toOpCode = map[MessageType]ws.OpCode{
	Text:   ws.OpText,
	Binary: ws.OpBinary,
	Ping:   ws.OpPing,
	Pong:   ws.OpPong,
	Close:  ws.OpClose,
}
