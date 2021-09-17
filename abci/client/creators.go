package abciclient

import (
	"fmt"

	"github.com/tendermint/tendermint/abci/types"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
)

//go:generate ../scripts/mockery_generate.sh Creator

// Creator creates new ABCI clients.
type Creator interface {
	// NewABCIClient returns a new ABCI client.
	NewABCIClient() (Client, error)
}

//----------------------------------------------------
// local proxy uses a mutex on an in-proc app

type localCreator struct {
	mtx *tmsync.RWMutex
	app types.Application
}

// NewLocalCreator returns a ClientCreator for the given app,
// which will be running locally.
func NewLocalCreator(app types.Application) Creator {
	return &localCreator{
		mtx: new(tmsync.RWMutex),
		app: app,
	}
}

func (l *localCreator) NewABCIClient() (Client, error) {
	return NewLocalClient(l.mtx, l.app), nil
}

//---------------------------------------------------------------
// remote proxy opens new connections to an external app process

type remoteCreator struct {
	addr        string
	transport   string
	mustConnect bool
}

// NewRemoteCreator returns a ClientCreator for the given address (e.g.
// "192.168.0.1") and transport (e.g. "tcp"). Set mustConnect to true if you
// want the client to connect before reporting success.
func NewRemoteCreator(addr, transport string, mustConnect bool) Creator {
	return &remoteCreator{
		addr:        addr,
		transport:   transport,
		mustConnect: mustConnect,
	}
}

func (r *remoteCreator) NewABCIClient() (Client, error) {
	remoteApp, err := NewClient(r.addr, r.transport, r.mustConnect)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	return remoteApp, nil
}
