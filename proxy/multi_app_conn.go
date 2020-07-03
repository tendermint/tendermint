package proxy

import (
	"fmt"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/libs/service"
)

// AppConns is the Tendermint's interface to the application that consists of
// multiple connections.
type AppConns interface {
	service.Service

	// Mempool connection
	Mempool() AppConnMempool
	// Consensus connection
	Consensus() AppConnConsensus
	// Query connection
	Query() AppConnQuery
	// Snapshot connection
	Snapshot() AppConnSnapshot
}

// NewAppConns calls NewMultiAppConn.
func NewAppConns(clientCreator ClientCreator) AppConns {
	return NewMultiAppConn(clientCreator)
}

// multiAppConn implements AppConns.
//
// A multiAppConn is made of a few appConns and manages their underlying abci
// clients.
// TODO: on app restart, clients must reboot together
type multiAppConn struct {
	service.BaseService

	consensusConn AppConnConsensus
	mempoolConn   AppConnMempool
	queryConn     AppConnQuery
	snapshotConn  AppConnSnapshot

	consensusConnClient abcicli.Client
	mempoolConnClient   abcicli.Client
	queryConnClient     abcicli.Client
	snapshotConnClient  abcicli.Client

	clientCreator ClientCreator
}

// NewMultiAppConn makes all necessary abci connections to the application.
func NewMultiAppConn(clientCreator ClientCreator) AppConns {
	multiAppConn := &multiAppConn{
		clientCreator: clientCreator,
	}
	multiAppConn.BaseService = *service.NewBaseService(nil, "multiAppConn", multiAppConn)
	return multiAppConn
}

func (app *multiAppConn) Mempool() AppConnMempool {
	return app.mempoolConn
}

func (app *multiAppConn) Consensus() AppConnConsensus {
	return app.consensusConn
}

func (app *multiAppConn) Query() AppConnQuery {
	return app.queryConn
}

func (app *multiAppConn) Snapshot() AppConnSnapshot {
	return app.snapshotConn
}

func (app *multiAppConn) OnStart() error {
	c, err := app.abciClientFor("query")
	if err != nil {
		return err
	}
	app.queryConnClient = c
	app.queryConn = NewAppConnQuery(c)

	c, err = app.abciClientFor("snapshot")
	if err != nil {
		app.stopAllClients()
		return err
	}
	app.snapshotConnClient = c
	app.snapshotConn = NewAppConnSnapshot(c)

	c, err = app.abciClientFor("mempool")
	if err != nil {
		app.stopAllClients()
		return err
	}
	app.mempoolConnClient = c
	app.mempoolConn = NewAppConnMempool(c)

	c, err = app.abciClientFor("consensus")
	if err != nil {
		app.stopAllClients()
		return err
	}
	app.consensusConnClient = c
	app.consensusConn = NewAppConnConsensus(c)

	return nil
}

func (app *multiAppConn) OnStop() {
	app.stopAllClients()
}

func (app *multiAppConn) stopAllClients() {
	if app.consensusConnClient != nil {
		app.consensusConnClient.Stop()
	}
	if app.mempoolConnClient != nil {
		app.mempoolConnClient.Stop()
	}
	if app.queryConnClient != nil {
		app.queryConnClient.Stop()
	}
	if app.snapshotConnClient != nil {
		app.snapshotConnClient.Stop()
	}
}

func (app *multiAppConn) abciClientFor(conn string) (abcicli.Client, error) {
	c, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return nil, fmt.Errorf("error creating ABCI client (%s connection): %w", conn, err)
	}
	c.SetLogger(app.Logger.With("module", "abci-client", "connection", conn))
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("error starting ABCI client (%s connection): %w", conn, err)
	}
	return c, nil
}
