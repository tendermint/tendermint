package proxy

import (
	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
)

//-----------------------------

// Tendermint's interface to the application consists of multiple connections
type AppConns interface {
	cmn.Service

	Mempool() AppConnMempool
	Consensus() AppConnConsensus
	Query() AppConnQuery
}

func NewAppConns(config cfg.Config, clientCreator ClientCreator, handshaker Handshaker) AppConns {
	return NewMultiAppConn(config, clientCreator, handshaker)
}

//-----------------------------
// multiAppConn implements AppConns

type Handshaker interface {
	Handshake(AppConns) error
}

// a multiAppConn is made of a few appConns (mempool, consensus, query)
// and manages their underlying abci clients, including the handshake
// which ensures the app and tendermint are synced.
// TODO: on app restart, clients must reboot together
type multiAppConn struct {
	cmn.BaseService

	config cfg.Config

	handshaker Handshaker

	mempoolConn   *appConnMempool
	consensusConn *appConnConsensus
	queryConn     *appConnQuery

	clientCreator ClientCreator
}

// Make all necessary abci connections to the application
func NewMultiAppConn(config cfg.Config, clientCreator ClientCreator, handshaker Handshaker) *multiAppConn {
	multiAppConn := &multiAppConn{
		config:        config,
		handshaker:    handshaker,
		clientCreator: clientCreator,
	}
	multiAppConn.BaseService = *cmn.NewBaseService(log, "multiAppConn", multiAppConn)
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

// Returns the query Connection
func (app *multiAppConn) Query() AppConnQuery {
	return app.queryConn
}

func (app *multiAppConn) OnStart() error {

	// query connection
	querycli, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return err
	}
	app.queryConn = NewAppConnQuery(querycli)

	// mempool connection
	memcli, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return err
	}
	app.mempoolConn = NewAppConnMempool(memcli)

	// consensus connection
	concli, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return err
	}
	app.consensusConn = NewAppConnConsensus(concli)

	// ensure app is synced to the latest state
	if app.handshaker != nil {
		return app.handshaker.Handshake(app)
	}
	return nil
}
