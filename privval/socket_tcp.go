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

// tcpTimeoutConn wraps a *net.TCPConn to standardise protocol timeouts / deadline resets.
type tcpTimeoutConn struct {
	*net.TCPConn

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

// newTCPTimeoutConn returns an instance of newTCPTimeoutConn.
func newTCPTimeoutConn(
	conn *net.TCPConn,
	connDeadline time.Duration) *tcpTimeoutConn {
	return &tcpTimeoutConn{
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

	// Wrap the TCPConn in our timeout wrapper
	conn := newTCPTimeoutConn(tc, ln.connDeadline)

	return conn, nil
}

// Read implements net.Listener.
func (c tcpTimeoutConn) Read(b []byte) (int, error) {
	// Reset deadline
	c.TCPConn.SetReadDeadline(time.Now().Add(c.connDeadline))

	return c.TCPConn.Read(b)
}

// Write implements net.Listener.
func (c tcpTimeoutConn) Write(b []byte) (int, error) {
	// Reset deadline
	c.TCPConn.SetWriteDeadline(time.Now().Add(c.connDeadline))

	return c.TCPConn.Write(b)
}
