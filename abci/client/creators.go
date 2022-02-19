package abciclient

import (
	"fmt"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
)

// Creator creates new ABCI clients.
type Creator func(log.Logger) (Client, error)

// NewLocalCreator returns a Creator for the given app,
// which will be running locally.
func NewLocalCreator(app types.Application) Creator {
	return func(logger log.Logger) (Client, error) {
		return NewLocalClient(logger, app), nil
	}
}

// NewRemoteCreator returns a Creator for the given address (e.g.
// "192.168.0.1") and transport (e.g. "tcp"). Set mustConnect to true if you
// want the client to connect before reporting success.
func NewRemoteCreator(logger log.Logger, addr, transport string, mustConnect bool) Creator {
	return func(log.Logger) (Client, error) {
		remoteApp, err := NewClient(logger, addr, transport, mustConnect)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to proxy: %w", err)
		}

		return remoteApp, nil
	}
}
