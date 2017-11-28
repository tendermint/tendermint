package proxy

import (
	"sync"

	"github.com/pkg/errors"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/abci/types"

	"github.com/tendermint/tmlibs/log"
)

// NewABCIClient returns newly connected client
type ClientCreator interface {
	NewABCIClient(log.Logger) (abcicli.Client, error)
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

func (l *localClientCreator) NewABCIClient(logger log.Logger) (abcicli.Client, error) {
	c := abcicli.NewLocalClient(l.mtx, l.app)
	c.BaseService.Logger = logger
	return c, nil
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

func (r *remoteClientCreator) NewABCIClient(logger log.Logger) (abcicli.Client, error) {
	remoteApp, err := abcicli.NewClient(r.addr, r.transport, r.mustConnect, logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to connect to proxy")
	}
	return remoteApp, nil
}

//-----------------------------------------------------------------
// default

func DefaultClientCreator(addr, transport, dbDir string) ClientCreator {
	switch addr {
	case "dummy":
		return NewLocalClientCreator(dummy.NewDummyApplication())
	case "persistent_dummy":
		return NewLocalClientCreator(dummy.NewPersistentDummyApplication(dbDir))
	case "nilapp":
		return NewLocalClientCreator(types.NewBaseApplication())
	default:
		mustConnect := false // loop retrying
		return NewRemoteClientCreator(addr, transport, mustConnect)
	}
}
