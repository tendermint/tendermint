package proxy

import (
	"fmt"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	tmsync "github.com/tendermint/tendermint/libs/sync"
)

// ClientCreator creates new ABCI clients.
type ClientCreator interface {
	// NewABCIClient returns a new ABCI client.
	NewABCIClient() (abcicli.Client, error)
}

//----------------------------------------------------
// local proxy uses a mutex on an in-proc app

type localClientCreator struct {
	mtx *tmsync.Mutex
	app types.Application
}

// NewLocalClientCreator returns a ClientCreator for the given app,
// which will be running locally.
func NewLocalClientCreator(app types.Application) ClientCreator {
	return &localClientCreator{
		mtx: new(tmsync.Mutex),
		app: app,
	}
}

func (l *localClientCreator) NewABCIClient() (abcicli.Client, error) {
	return abcicli.NewLocalClient(l.mtx, l.app), nil
}

//---------------------------------------------------------------
// remote proxy opens new connections to an external app process

type remoteClientCreator struct {
	addr        string
	transport   string
	mustConnect bool
}

// NewRemoteClientCreator returns a ClientCreator for the given address (e.g.
// "192.168.0.1") and transport (e.g. "tcp"). Set mustConnect to true if you
// want the client to connect before reporting success.
func NewRemoteClientCreator(addr, transport string, mustConnect bool) ClientCreator {
	return &remoteClientCreator{
		addr:        addr,
		transport:   transport,
		mustConnect: mustConnect,
	}
}

func (r *remoteClientCreator) NewABCIClient() (abcicli.Client, error) {
	remoteApp, err := abcicli.NewClient(r.addr, r.transport, r.mustConnect)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	return remoteApp, nil
}

// DefaultClientCreator returns a default ClientCreator, which will create a
// local client if addr is one of: 'counter', 'counter_serial', 'kvstore',
// 'persistent_kvstore' or 'noop', otherwise - a remote client.
func DefaultClientCreator(addr, transport, dbDir string) ClientCreator {
	switch addr {
	case "counter":
		return NewLocalClientCreator(counter.NewApplication(false))
	case "counter_serial":
		return NewLocalClientCreator(counter.NewApplication(true))
	case "kvstore":
		return NewLocalClientCreator(kvstore.NewApplication())
	case "persistent_kvstore":
		return NewLocalClientCreator(kvstore.NewPersistentKVStoreApplication(dbDir))
	case "noop":
		return NewLocalClientCreator(types.NewBaseApplication())
	default:
		mustConnect := false // loop retrying
		return NewRemoteClientCreator(addr, transport, mustConnect)
	}
}
