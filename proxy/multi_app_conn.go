package proxy

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/service"
)

//-----------------------------

// Tendermint's interface to the application consists of multiple connections
type AppConns interface {
	service.Service

	Mempool() AppConnMempool
	Consensus() AppConnConsensus
	Query() AppConnQuery
	Snapshot() AppConnSnapshot
}

func NewAppConns(clientCreator ClientCreator) AppConns {
	return NewMultiAppConn(clientCreator)
}

//-----------------------------
// multiAppConn implements AppConns

// a multiAppConn is made of a few appConns (mempool, consensus, query)
// and manages their underlying abci clients
// TODO: on app restart, clients must reboot together
type multiAppConn struct {
	service.BaseService

	mempoolConn   AppConnMempool
	consensusConn AppConnConsensus
	queryConn     AppConnQuery
	snapshotConn  AppConnSnapshot

	clientCreator ClientCreator
}

// Make all necessary abci connections to the application
func NewMultiAppConn(clientCreator ClientCreator) AppConns {
	multiAppConn := &multiAppConn{
		clientCreator: clientCreator,
	}
	multiAppConn.BaseService = *service.NewBaseService(nil, "multiAppConn", multiAppConn)
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

// Returns the snapshot Connection
func (app *multiAppConn) Snapshot() AppConnSnapshot {
	return app.snapshotConn
}

func (app *multiAppConn) OnStart() error {
	// query connection
	querycli, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return fmt.Errorf("error creating ABCI client (query connection): %w", err)
	}
	querycli.SetLogger(app.Logger.With("module", "abci-client", "connection", "query"))
	if err := querycli.Start(); err != nil {
		return fmt.Errorf("error starting ABCI client (query connection): %w", err)
	}
	app.queryConn = NewAppConnQuery(querycli)

	// snapshot connection
	snapshotcli, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return fmt.Errorf("error creating ABCI client (snapshot connection): %w", err)
	}
	snapshotcli.SetLogger(app.Logger.With("module", "abci-client", "connection", "snapshot"))
	if err := snapshotcli.Start(); err != nil {
		return fmt.Errorf("error starting ABCI client (snapshot connection): %w", err)
	}
	app.snapshotConn = NewAppConnSnapshot(snapshotcli)

	// mempool connection
	memcli, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return fmt.Errorf("error creating ABCI client (mempool connection): %w", err)
	}
	memcli.SetLogger(app.Logger.With("module", "abci-client", "connection", "mempool"))
	if err := memcli.Start(); err != nil {
		return fmt.Errorf("error starting ABCI client (mempool connection): %w", err)
	}
	app.mempoolConn = NewAppConnMempool(memcli)

	// consensus connection
	concli, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return fmt.Errorf("error creating ABCI client (consensus connection): %w", err)
	}
	concli.SetLogger(app.Logger.With("module", "abci-client", "connection", "consensus"))
	if err := concli.Start(); err != nil {
		return fmt.Errorf("error starting ABCI client (consensus connection): %w", err)
	}
	app.consensusConn = NewAppConnConsensus(concli)

	return nil
}
