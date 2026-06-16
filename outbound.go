package tnet

// OutboundBuffered returns the current outbound buffered bytes for conn.
// It returns 0 if conn is nil or is not backed by tnet TCP.
func OutboundBuffered(conn Conn) int {
	tc, ok := conn.(*tcpconn)
	if !ok || tc == nil {
		return 0
	}
	return tc.outBuffer.LenRead()
}
