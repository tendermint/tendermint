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

	if err := tc.SetDeadline(time.Now().Add(ln.connDeadline)); err != nil {
		return nil, err
	}

	if err := tc.SetKeepAlive(true); err != nil {
		return nil, err
	}

	if err := tc.SetKeepAlivePeriod(ln.period); err != nil {
		return nil, err
	}

	return tc, nil
}
