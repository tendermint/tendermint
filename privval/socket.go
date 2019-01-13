package privval

import (
	"net"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
)

var (
	acceptDeadline = time.Second * defaultAcceptDeadlineSeconds
	connTimeout    = time.Second * defaultConnDeadlineSeconds
)

// timeoutError can be used to check if an error returned from the netp package
// was due to a timeout.
type timeoutError interface {
	Timeout() bool
}

//------------------------------------------------------------------
// TCP Listener

// tcpListener implements net.Listener.
var _ net.Listener = (*tcpListener)(nil)

// tcpListener wraps a *net.TCPListener to standardise protocol timeouts
// and potentially other tuning parameters. It also returns encrypted connections.
type tcpListener struct {
	*net.TCPListener

	secretConnKey ed25519.PrivKeyEd25519

	acceptDeadline time.Duration
	connDeadline   time.Duration
}

// newTCPListener returns an instance of tcpListener.
func NewTCPListener(
	ln net.Listener,
	acceptDeadline, connDeadline time.Duration,
	secretConnKey ed25519.PrivKeyEd25519,
) tcpListener {
	return tcpListener{
		TCPListener:    ln.(*net.TCPListener),
		acceptDeadline: acceptDeadline,
		connDeadline:   connDeadline,
		secretConnKey:  secretConnKey,
	}
}

// Accept implements net.Listener.
func (ln tcpListener) Accept() (net.Conn, error) {
	err := ln.SetDeadline(time.Now().Add(ln.acceptDeadline))
	if err != nil {
		return nil, err
	}

	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// Wrap the conn in our timeout and encryption wrappers
	timeoutConn := newTimeoutConn(tc, ln.connDeadline)
	secretConn, err := p2pconn.MakeSecretConnection(timeoutConn, ln.secretConnKey)
	if err != nil {
		return nil, err
	}

	return secretConn, nil
}

//------------------------------------------------------------------
// Unix Listener

// unixListener implements net.Listener.
var _ net.Listener = (*unixListener)(nil)

// unixListener wraps a *net.UnixListener to standardise protocol timeouts
// and potentially other tuning parameters. It returns unencrypted connections.
type unixListener struct {
	*net.UnixListener

	acceptDeadline time.Duration
	connDeadline   time.Duration
}

// NewUnixListener returns an instance of unixListener.
func NewUnixListener(
	ln net.Listener,
	acceptDeadline, connDeadline time.Duration,
) unixListener {
	return unixListener{
		UnixListener:   ln.(*net.UnixListener),
		acceptDeadline: acceptDeadline,
		connDeadline:   connDeadline,
	}
}

// Accept implements net.Listener.
func (ln unixListener) Accept() (net.Conn, error) {
	err := ln.SetDeadline(time.Now().Add(ln.acceptDeadline))
	if err != nil {
		return nil, err
	}

	tc, err := ln.AcceptUnix()
	if err != nil {
		return nil, err
	}

	// Wrap the conn in our timeout wrapper
	conn := newTimeoutConn(tc, ln.connDeadline)

	// TODO: wrap in something that authenticates
	// with a MAC - https://github.com/tendermint/tendermint/issues/3099

	return conn, nil
}

//------------------------------------------------------------------
// Connection

// timeoutConn implements net.Conn.
var _ net.Conn = (*timeoutConn)(nil)

// timeoutConn wraps a net.Conn to standardise protocol timeouts / deadline resets.
type timeoutConn struct {
	net.Conn

	connDeadline time.Duration
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

// Read implements net.Conn.
func (c timeoutConn) Read(b []byte) (n int, err error) {
	// Reset deadline
	c.Conn.SetReadDeadline(time.Now().Add(c.connDeadline))

	return c.Conn.Read(b)
}

// Write implements net.Conn.
func (c timeoutConn) Write(b []byte) (n int, err error) {
	// Reset deadline
	c.Conn.SetWriteDeadline(time.Now().Add(c.connDeadline))

	return c.Conn.Write(b)
}
