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

func NewRemoteAppConn(addr, transport string) (*remoteAppConn, error) {
	mustConnect := false // force reconnect attempts
	client, err := tmspcli.NewClient(addr, transport, mustConnect)
	if err != nil {
		return nil, err
	}
	appConn := &remoteAppConn{
		Client: client,
	}
	return appConn, nil
}
