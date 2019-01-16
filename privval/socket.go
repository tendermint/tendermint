package privval

import (
	"net"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
)

const (
	defaultAcceptDeadlineSeconds = 3
	defaultConnDeadlineSeconds   = 3
)

// timeoutError can be used to check if an error returned from the netp package
// was due to a timeout.
type timeoutError interface {
	Timeout() bool
}

//------------------------------------------------------------------
// TCP Listener

// TCPListenerOption sets an optional parameter on the tcpListener.
type TCPListenerOption func(*tcpListener)

// TCPListenerAcceptDeadline sets the deadline for the listener.
// A zero time value disables the deadline.
func TCPListenerAcceptDeadline(deadline time.Duration) TCPListenerOption {
	return func(tl *tcpListener) { tl.acceptDeadline = deadline }
}

// TCPListenerConnDeadline sets the read and write deadline for connections
// from external signing processes.
func TCPListenerConnDeadline(deadline time.Duration) TCPListenerOption {
	return func(tl *tcpListener) { tl.connDeadline = deadline }
}

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

// NewTCPListener returns a listener that accepts authenticated encrypted connections
// using the given secretConnKey and the default timeout values.
func NewTCPListener(ln net.Listener, secretConnKey ed25519.PrivKeyEd25519) *tcpListener {
	return &tcpListener{
		TCPListener:    ln.(*net.TCPListener),
		secretConnKey:  secretConnKey,
		acceptDeadline: time.Second * defaultAcceptDeadlineSeconds,
		connDeadline:   time.Second * defaultConnDeadlineSeconds,
	}
}

// Accept implements net.Listener.
func (ln *tcpListener) Accept() (net.Conn, error) {
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

type UnixListenerOption func(*unixListener)

// UnixListenerAcceptDeadline sets the deadline for the listener.
// A zero time value disables the deadline.
func UnixListenerAcceptDeadline(deadline time.Duration) UnixListenerOption {
	return func(ul *unixListener) { ul.acceptDeadline = deadline }
}

// UnixListenerConnDeadline sets the read and write deadline for connections
// from external signing processes.
func UnixListenerConnDeadline(deadline time.Duration) UnixListenerOption {
	return func(ul *unixListener) { ul.connDeadline = deadline }
}

// unixListener wraps a *net.UnixListener to standardise protocol timeouts
// and potentially other tuning parameters. It returns unencrypted connections.
type unixListener struct {
	*net.UnixListener

	acceptDeadline time.Duration
	connDeadline   time.Duration
}

// NewUnixListener returns a listener that accepts unencrypted connections
// using the default timeout values.
func NewUnixListener(ln net.Listener) *unixListener {
	return &unixListener{
		UnixListener:   ln.(*net.UnixListener),
		acceptDeadline: time.Second * defaultAcceptDeadlineSeconds,
		connDeadline:   time.Second * defaultConnDeadlineSeconds,
	}
}

// Accept implements net.Listener.
func (ln *unixListener) Accept() (net.Conn, error) {
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

// newTimeoutConn returns an instance of timeoutConn.
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
