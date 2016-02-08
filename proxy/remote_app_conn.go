package proxy

import (
	tmspcli "github.com/tendermint/tmsp/client"
)

// This is goroutine-safe, but users should beware that
// the application in general is not meant to be interfaced
// with concurrent callers.
type remoteAppConn struct {
	*tmspcli.TMSPClient
}

func NewRemoteAppConn(addr string) (*remoteAppConn, error) {
	client, err := tmspcli.NewTMSPClient(addr)
	if err != nil {
		return nil, err
	}
	appConn := &remoteAppConn{
		TMSPClient: client,
	}
	return appConn, nil
}
