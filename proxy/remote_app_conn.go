package proxy

import (
	tmspcli "github.com/tendermint/tmsp/client"
)

// This is goroutine-safe, but users should beware that
// the application in general is not meant to be interfaced
// with concurrent callers.
type remoteAppConn struct {
	tmspcli.Client
}

func NewRemoteAppConn(addr string) (*remoteAppConn, error) {
	client, err := tmspcli.NewClient(addr, false)
	if err != nil {
		return nil, err
	}
	appConn := &remoteAppConn{
		Client: client,
	}
	return appConn, nil
}
