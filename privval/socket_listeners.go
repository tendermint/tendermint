package privval

import (
	"net"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
)

const (
	defaultTimeoutAcceptSeconds    = 3
	defaultTimeoutReadWriteSeconds = 3
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

// TCPListenerTimeoutAccept sets the timeout for the listener.
// A zero time value disables the timeout.
func TCPListenerTimeoutAccept(timeout time.Duration) TCPListenerOption {
	return func(tl *tcpListener) { tl.timeoutAccept = timeout }
}

// TCPListenerTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func TCPListenerTimeoutReadWrite(timeout time.Duration) TCPListenerOption {
	return func(tl *tcpListener) { tl.timeoutReadWrite = timeout }
}

// tcpListener implements net.Listener.
var _ net.Listener = (*tcpListener)(nil)

// tcpListener wraps a *net.TCPListener to standardise protocol timeouts
// and potentially other tuning parameters. It also returns encrypted connections.
type tcpListener struct {
	*net.TCPListener

	secretConnKey ed25519.PrivKeyEd25519

	timeoutAccept    time.Duration
	timeoutReadWrite time.Duration
}

// NewTCPListener returns a listener that accepts authenticated encrypted connections
// using the given secretConnKey and the default timeout values.
func NewTCPListener(ln net.Listener, secretConnKey ed25519.PrivKeyEd25519) *tcpListener {
	return &tcpListener{
		TCPListener:      ln.(*net.TCPListener),
		secretConnKey:    secretConnKey,
		timeoutAccept:    time.Second * defaultTimeoutAcceptSeconds,
		timeoutReadWrite: time.Second * defaultTimeoutReadWriteSeconds,
	}
}

// Accept implements net.Listener.
func (ln *tcpListener) Accept() (net.Conn, error) {
	deadline := time.Now().Add(ln.timeoutAccept)
	err := ln.SetDeadline(deadline)
	if err != nil {
		return nil, err
	}

	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// Wrap the conn in our timeout and encryption wrappers
	timeoutConn := newTimeoutConn(tc, ln.timeoutReadWrite)
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

// UnixListenerTimeoutAccept sets the timeout for the listener.
// A zero time value disables the timeout.
func UnixListenerTimeoutAccept(timeout time.Duration) UnixListenerOption {
	return func(ul *unixListener) { ul.timeoutAccept = timeout }
}

// UnixListenerTimeoutReadWrite sets the read and write timeout for connections
// from external signing processes.
func UnixListenerTimeoutReadWrite(timeout time.Duration) UnixListenerOption {
	return func(ul *unixListener) { ul.timeoutReadWrite = timeout }
}

// unixListener wraps a *net.UnixListener to standardise protocol timeouts
// and potentially other tuning parameters. It returns unencrypted connections.
type unixListener struct {
	*net.UnixListener

	timeoutAccept    time.Duration
	timeoutReadWrite time.Duration
}

// NewUnixListener returns a listener that accepts unencrypted connections
// using the default timeout values.
func NewUnixListener(ln net.Listener) *unixListener {
	return &unixListener{
		UnixListener:     ln.(*net.UnixListener),
		timeoutAccept:    time.Second * defaultTimeoutAcceptSeconds,
		timeoutReadWrite: time.Second * defaultTimeoutReadWriteSeconds,
	}
}

// Accept implements net.Listener.
func (ln *unixListener) Accept() (net.Conn, error) {
	deadline := time.Now().Add(ln.timeoutAccept)
	err := ln.SetDeadline(deadline)
	if err != nil {
		return nil, err
	}

	tc, err := ln.AcceptUnix()
	if err != nil {
		return nil, err
	}

	// Wrap the conn in our timeout wrapper
	conn := newTimeoutConn(tc, ln.timeoutReadWrite)

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
	timeout time.Duration
}

// newTimeoutConn returns an instance of timeoutConn.
func newTimeoutConn(conn net.Conn, timeout time.Duration) *timeoutConn {
	return &timeoutConn{
		conn,
		timeout,
	}
}

// Read implements net.Conn.
func (c timeoutConn) Read(b []byte) (n int, err error) {
	// Reset deadline
	deadline := time.Now().Add(c.timeout)
	err = c.Conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}

	return c.Conn.Read(b)
}

// Write implements net.Conn.
func (c timeoutConn) Write(b []byte) (n int, err error) {
	// Reset deadline
	deadline := time.Now().Add(c.timeout)
	err = c.Conn.SetWriteDeadline(deadline)
	if err != nil {
		return
	}

	return c.Conn.Write(b)
}
