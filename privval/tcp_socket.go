package privval

import (
	"net"
	"time"
)

// timeoutError can be used to check if an error returned from the netp package
// was due to a timeout.
type timeoutError interface {
	Timeout() bool
}

// tcpTimeoutListener implements net.Listener.
var _ net.Listener = (*tcpTimeoutListener)(nil)

// tcpTimeoutListener wraps a *net.TCPListener to standardise protocol timeouts
// and potentially other tuning parameters.
type tcpTimeoutListener struct {
	*net.TCPListener

	acceptDeadline time.Duration
	connDeadline   time.Duration
	period         time.Duration
}

// timeoutConn wraps a net.Conn to standardise protocol timeouts / deadline resets.
type timeoutConn struct {
	net.Conn

	connDeadline time.Duration
}

// newTCPTimeoutListener returns an instance of tcpTimeoutListener.
func newTCPTimeoutListener(
	ln net.Listener,
	acceptDeadline, connDeadline time.Duration,
	period time.Duration,
) tcpTimeoutListener {
	return tcpTimeoutListener{
		TCPListener:    ln.(*net.TCPListener),
		acceptDeadline: acceptDeadline,
		connDeadline:   connDeadline,
		period:         period,
	}
}

// newTimeoutConn returns an instance of newTCPTimeoutConn.
func newTimeoutConn(
	conn net.Conn,
	connDeadline time.Duration) *timeoutConn {
	return &timeoutConn{
		conn,
		connDeadline,
	}
}

// Accept implements net.Listener.
func (ln tcpTimeoutListener) Accept() (net.Conn, error) {
	err := ln.SetDeadline(time.Now().Add(ln.acceptDeadline))
	if err != nil {
		return nil, err
	}

	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// Wrap the conn in our timeout wrapper
	conn := newTimeoutConn(tc, ln.connDeadline)

	return conn, nil
}

// Read implements net.Listener.
func (c timeoutConn) Read(b []byte) (n int, err error) {
	// Reset deadline
	c.Conn.SetReadDeadline(time.Now().Add(c.connDeadline))

	return c.Conn.Read(b)
}

// Write implements net.Listener.
func (c timeoutConn) Write(b []byte) (n int, err error) {
	// Reset deadline
	c.Conn.SetWriteDeadline(time.Now().Add(c.connDeadline))

	return c.Conn.Write(b)
}
