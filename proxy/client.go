package proxy

import (
	"fmt"
	"sync"

	cfg "github.com/tendermint/go-config"
	tmspcli "github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/example/dummy"
	nilapp "github.com/tendermint/tmsp/example/nil"
	"github.com/tendermint/tmsp/types"
)

// NewTMSPClient returns newly connected client
type ClientCreator interface {
	NewTMSPClient() (tmspcli.Client, error)
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

func (l *localClientCreator) NewTMSPClient() (tmspcli.Client, error) {
	return tmspcli.NewLocalClient(l.mtx, l.app), nil
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

func (r *remoteClientCreator) NewTMSPClient() (tmspcli.Client, error) {
	// Run forever in a loop
	remoteApp, err := tmspcli.NewClient(r.addr, r.transport, r.mustConnect)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to proxy: %v", err)
	}
	return remoteApp, nil
}

//-----------------------------------------------------------------
// default

func DefaultClientCreator(config cfg.Config) ClientCreator {
	addr := config.GetString("proxy_app")
	transport := config.GetString("tmsp")

	switch addr {
	case "dummy":
		return NewLocalClientCreator(dummy.NewDummyApplication())
	case "nilapp":
		return NewLocalClientCreator(nilapp.NewNilApplication())
	default:
		mustConnect := false // loop retrying
		return NewRemoteClientCreator(addr, transport, mustConnect)
	}
}
