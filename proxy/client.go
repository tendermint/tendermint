package proxy

import (
	"fmt"
	"sync"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/abci/types"
	cfg "github.com/tendermint/go-config"
)

// NewABCIClient returns newly connected client
type ClientCreator interface {
	NewABCIClient() (abcicli.Client, error)
}

//----------------------------------------------------
// local proxy uses a mutex on an in-proc app

type localClientCreator struct {
	mtx *sync.Mutex
	app types.Application
}

func NewLocalClientCreator(app types.Application) ClientCreator {
	return &localClientCreator{
		mtx: new(sync.Mutex),
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

func NewRemoteClientCreator(addr, transport string, mustConnect bool) ClientCreator {
	return &remoteClientCreator{
		addr:        addr,
		transport:   transport,
		mustConnect: mustConnect,
	}
}

func (r *remoteClientCreator) NewABCIClient() (abcicli.Client, error) {
	// Run forever in a loop
	remoteApp, err := abcicli.NewClient(r.addr, r.transport, r.mustConnect)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to proxy: %v", err)
	}
	return remoteApp, nil
}

//-----------------------------------------------------------------
// default

func DefaultClientCreator(config cfg.Config) ClientCreator {
	addr := config.GetString("proxy_app")
	transport := config.GetString("abci")

	switch addr {
	case "dummy":
		return NewLocalClientCreator(dummy.NewDummyApplication())
	case "persistent_dummy":
		return NewLocalClientCreator(dummy.NewPersistentDummyApplication(config.GetString("db_dir")))
	case "nilapp":
		return NewLocalClientCreator(types.NewBaseApplication())
	default:
		mustConnect := false // loop retrying
		return NewRemoteClientCreator(addr, transport, mustConnect)
	}
}
