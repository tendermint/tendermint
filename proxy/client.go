package proxy

import (
	"fmt"
	"sync"

	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abcix/adapter"
	kvstorex "github.com/tendermint/tendermint/abcix/example/kvstore"

	abcixcli "github.com/tendermint/tendermint/abcix/client"
	abcix "github.com/tendermint/tendermint/abcix/types"
)

// ClientCreator creates new ABCI clients.
type ClientCreator interface {
	// NewABCIClient returns a new ABCI client.
	NewABCIClient() (abcixcli.Client, error)
}

//----------------------------------------------------
// local proxy uses a mutex on an in-proc app

type localClientCreator struct {
	mtx *sync.Mutex
	app abcix.Application
}

// NewLocalClientCreator returns a ClientCreator for the given app,
// which will be running locally.
func NewLocalClientCreator(app abcix.Application) ClientCreator {
	return &localClientCreator{
		mtx: new(sync.Mutex),
		app: app,
	}
}

func (l *localClientCreator) NewABCIClient() (abcixcli.Client, error) {
	return abcixcli.NewLocalClient(l.mtx, l.app), nil
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

func (r *remoteClientCreator) NewABCIClient() (abcixcli.Client, error) {
	remoteApp, err := abcixcli.NewClient(r.addr, r.transport, r.mustConnect)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}
	return remoteApp, nil
}

// DefaultClientCreator returns a default ClientCreator, which will create a
// local client if addr is one of: 'counter', 'counter_serial', 'kvstore',
// 'persistent_kvstore' or 'noop', otherwise - a remote client.
// TODO: will implement some of existing ABCI apps in ABCIx later
func DefaultClientCreator(addr, transport, dbDir string) ClientCreator {
	switch addr {
	case "counter":
		return NewLocalClientCreator(adapter.AdaptToABCIx(counter.NewApplication(false)))
	case "counter_serial":
		return NewLocalClientCreator(adapter.AdaptToABCIx(counter.NewApplication(true)))
	case "kvstore":
		return NewLocalClientCreator(kvstorex.NewApplication())
	case "persistent_kvstore":
		return NewLocalClientCreator(adapter.AdaptToABCIx(kvstore.NewPersistentKVStoreApplication(dbDir)))
	case "noop":
		return NewLocalClientCreator(abcix.NewBaseApplication())
	default:
		mustConnect := false // loop retrying
		return NewRemoteClientCreator(addr, transport, mustConnect)
	}
}
