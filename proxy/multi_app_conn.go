package proxy

import (
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
)

// Tendermint's interface to the application consists of multiple connections
type AppConns interface {
	Service

	Mempool() AppConnMempool
	Consensus() AppConnConsensus
	Query() AppConnQuery
}

func NewAppConns(config cfg.Config, clientCreator ClientCreator, state State, blockStore BlockStore) AppConns {
	return NewMultiAppConn(config, clientCreator, state, blockStore)
}

// a multiAppConn is made of a few appConns (mempool, consensus, query)
// and manages their underlying tmsp clients, ensuring they reboot together
type multiAppConn struct {
	QuitService

	config cfg.Config

	state      State
	blockStore BlockStore

	mempoolConn   *appConnMempool
	consensusConn *appConnConsensus
	queryConn     *appConnQuery

	clientCreator ClientCreator
}

// Make all necessary tmsp connections to the application
func NewMultiAppConn(config cfg.Config, clientCreator ClientCreator, state State, blockStore BlockStore) *multiAppConn {
	multiAppConn := &multiAppConn{
		config:        config,
		state:         state,
		blockStore:    blockStore,
		clientCreator: clientCreator,
	}
	multiAppConn.QuitService = *NewQuitService(log, "multiAppConn", multiAppConn)
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

	// query connection
	querycli, err := app.clientCreator.NewTMSPClient()
	if err != nil {
		return err
	}
	app.queryConn = NewAppConnQuery(querycli)

	// mempool connection
	memcli, err := app.clientCreator.NewTMSPClient()
	if err != nil {
		return err
	}
	app.mempoolConn = NewAppConnMempool(memcli)

	// consensus connection
	concli, err := app.clientCreator.NewTMSPClient()
	if err != nil {
		return err
	}
	app.consensusConn = NewAppConnConsensus(concli)

	// TODO: handshake

	// TODO: replay blocks

	// TODO: (on restart) replay mempool

	return nil
}
