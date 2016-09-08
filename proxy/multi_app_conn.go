package proxy

import (
	"fmt"
	"sync"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	tmspcli "github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/example/dummy"
	nilapp "github.com/tendermint/tmsp/example/nil"
)

// Get a connected tmsp client
func NewTMSPClient(addr, transport string) (tmspcli.Client, error) {
	var client tmspcli.Client

	// use local app (for testing)
	// TODO: local proxy app conn
	switch addr {
	case "nilapp":
		app := nilapp.NewNilApplication()
		mtx := new(sync.Mutex) // TODO
		client = tmspcli.NewLocalClient(mtx, app)
	case "dummy":
		app := dummy.NewDummyApplication()
		mtx := new(sync.Mutex) // TODO
		client = tmspcli.NewLocalClient(mtx, app)
	default:
		// Run forever in a loop
		mustConnect := false
		remoteApp, err := tmspcli.NewClient(addr, transport, mustConnect)
		if err != nil {
			return nil, fmt.Errorf("Failed to connect to proxy for mempool: %v", err)
		}
		client = remoteApp
	}
	return client, nil
}

//---------

type AppConns interface {
	Mempool() AppConnMempool
	Consensus() AppConnConsensus
	Query() AppConnQuery
}

func NewAppConns(config cfg.Config, state State, blockStore BlockStore) AppConns {
	return NewMultiAppConn(config, state, blockStore)
}

// a multiAppConn is made of a few appConns (mempool, consensus)
// and manages their underlying tmsp clients, ensuring they reboot together
type multiAppConn struct {
	QuitService

	config cfg.Config

	state      State
	blockStore BlockStore

	mempoolConn   *appConnMempool
	consensusConn *appConnConsensus
	queryConn     *appConnQuery
}

// Make all necessary tmsp connections to the application
func NewMultiAppConn(config cfg.Config, state State, blockStore BlockStore) *multiAppConn {
	multiAppConn := &multiAppConn{
		config:     config,
		state:      state,
		blockStore: blockStore,
	}
	multiAppConn.QuitService = *NewQuitService(log, "multiAppConn", multiAppConn)
	multiAppConn.Start()
	return multiAppConn
}

// Returns the mempool connection
func (app *multiAppConn) Mempool() AppConnMempool {
	return app.mempoolConn
}

// Returns the consensus Connection
func (app *multiAppConn) Consensus() AppConnConsensus {
	return app.consensusConn
}

func (app *multiAppConn) Query() AppConnQuery {
	return app.queryConn
}

func (app *multiAppConn) OnStart() error {
	app.QuitService.OnStart()

	addr := app.config.GetString("proxy_app")
	transport := app.config.GetString("tmsp")

	// query connection
	querycli, err := NewTMSPClient(addr, transport)
	if err != nil {
		return err
	}
	app.queryConn = NewAppConnQuery(querycli)

	// mempool connection
	memcli, err := NewTMSPClient(addr, transport)
	if err != nil {
		return err
	}
	app.mempoolConn = NewAppConnMempool(memcli)

	// consensus connection
	concli, err := NewTMSPClient(addr, transport)
	if err != nil {
		return err
	}
	app.consensusConn = NewAppConnConsensus(concli)

	// TODO: handshake

	// TODO: replay blocks

	// TODO: (on restart) replay mempool

	return nil
}
