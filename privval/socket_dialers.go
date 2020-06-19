package privval

import (
	"errors"
	"net"
	"time"

	"github.com/tendermint/tendermint/crypto"
	tmnet "github.com/tendermint/tendermint/libs/net"
	p2pconn "github.com/tendermint/tendermint/p2p/conn"
)

// Socket errors.
var (
	ErrDialRetryMax = errors.New("dialed maximum retries")
)

// SocketDialer dials a remote address and returns a net.Conn or an error.
type SocketDialer func() (net.Conn, error)

// DialTCPFn dials the given tcp addr, using the given timeoutReadWrite and
// privKey for the authenticated encryption handshake.
func DialTCPFn(addr string, timeoutReadWrite time.Duration, privKey crypto.PrivKey) SocketDialer {
	return func() (net.Conn, error) {
		conn, err := tmnet.Connect(addr)
		if err == nil {
			deadline := time.Now().Add(timeoutReadWrite)
			err = conn.SetDeadline(deadline)
		}
		if err == nil {
			conn, err = p2pconn.MakeSecretConnection(conn, privKey)
		}
		return conn, err
	}
}

// DialUnixFn dials the given unix socket.
func DialUnixFn(addr string) SocketDialer {
	return func() (net.Conn, error) {
		unixAddr := &net.UnixAddr{Name: addr, Net: "unix"}
		return net.DialUnix("unix", nil, unixAddr)
	}
}
