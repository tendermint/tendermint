package proxy

import (
	"fmt"
	"sync"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
)

// LegacyClientCreator creates new ABCI clients.
type LegacyClientCreator interface {
	// NewABCIClient returns a new ABCI client.
	NewABCIClient() (abcicli.Client, error)
}

//----------------------------------------------------
// local proxy uses a mutex on an in-proc app

type legacyLocalClientCreator struct {
	mtx *sync.Mutex
	app types.Application
}

// NewLocalClientCreator returns a ClientCreator for the given app,
// which will be running locally.
func NewLegacyLocalClientCreator(app types.Application) LegacyClientCreator {
	return &legacyLocalClientCreator{
		mtx: new(sync.Mutex),
		app: app,
	}
}

func (l *legacyLocalClientCreator) NewABCIClient() (abcicli.Client, error) {
	return abcicli.NewLocalClient(l.mtx, l.app), nil
}

//---------------------------------------------------------------
// remote proxy opens new connections to an external app process

type legacyRemoteClientCreator struct {
	addr        string
	transport   string
	mustConnect bool
}

// NewRemoteClientCreator returns a ClientCreator for the given address (e.g.
// "192.168.0.1") and transport (e.g. "tcp"). Set mustConnect to true if you
// want the client to connect before reporting success.
func NewLegacyRemoteClientCreator(addr, transport string, mustConnect bool) LegacyClientCreator {
	return &legacyRemoteClientCreator{
		addr:        addr,
		transport:   transport,
		mustConnect: mustConnect,
	}
}

func (r *legacyRemoteClientCreator) NewABCIClient() (abcicli.Client, error) {
	remoteApp, err := abcicli.NewClient(r.addr, r.transport, r.mustConnect)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}
	return remoteApp, nil
}

// DefaultClientCreator returns a default ClientCreator, which will create a
// local client if addr is one of: 'counter', 'counter_serial', 'kvstore',
// 'persistent_kvstore' or 'noop', otherwise - a remote client.
func DefaultLegacyClientCreator(addr, transport, dbDir string) LegacyClientCreator {
	switch addr {
	case "counter":
		return NewLegacyLocalClientCreator(counter.NewApplication(false))
	case "counter_serial":
		return NewLegacyLocalClientCreator(counter.NewApplication(true))
	case "kvstore":
		return NewLegacyLocalClientCreator(kvstore.NewApplication())
	case "persistent_kvstore":
		return NewLegacyLocalClientCreator(kvstore.NewPersistentKVStoreApplication(dbDir))
	case "noop":
		return NewLegacyLocalClientCreator(types.NewBaseApplication())
	default:
		mustConnect := false // loop retrying
		return NewLegacyRemoteClientCreator(addr, transport, mustConnect)
	}
}
