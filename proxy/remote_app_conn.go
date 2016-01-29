package proxy

import (
	"net"

	tmspcli "github.com/tendermint/tmsp/client"
)

// This is goroutine-safe, but users should beware that
// the application in general is not meant to be interfaced
// with concurrent callers.
type remoteAppConn struct {
	*tmspcli.TMSPClient
}

func NewRemoteAppConn(conn net.Conn, bufferSize int) *remoteAppConn {
	app := &remoteAppConn{
		TMSPClient: tmspcli.NewTMSPClient(conn, bufferSize),
	}
	return app
}
