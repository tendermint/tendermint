package abciclient

import (
	"fmt"

	"github.com/tendermint/tendermint/abci/types"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
)

// Creator creates new ABCI clients.
type Creator func() (Client, error)

// NewLocalCreator returns a Creator for the given app,
// which will be running locally.
func NewLocalCreator(app types.Application) Creator {
	mtx := new(tmsync.Mutex)

	return func() (Client, error) {
		return NewLocalClient(mtx, app), nil
	}
}

// NewRemoteCreator returns a Creator for the given address (e.g.
// "192.168.0.1") and transport (e.g. "tcp"). Set mustConnect to true if you
// want the client to connect before reporting success.
func NewRemoteCreator(addr, transport string, mustConnect bool) Creator {
	return func() (Client, error) {
		remoteApp, err := NewClient(addr, transport, mustConnect)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to proxy: %w", err)
		}

		return remoteApp, nil
	}
}
