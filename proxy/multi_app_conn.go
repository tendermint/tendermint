package proxy

import (
	"github.com/pkg/errors"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

//-----------------------------

// Tendermint's interface to the application consists of multiple connections
type AppConns interface {
	cmn.Service

	Mempool() AppConnMempool
	Consensus() AppConnConsensus
	Query() AppConnQuery
}

func NewAppConns(clientCreator ClientCreator, handshaker Handshaker, logger log.Logger) AppConns {
	return NewMultiAppConn(clientCreator, handshaker, logger)
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

	handshaker Handshaker

	mempoolConn   *appConnMempool
	consensusConn *appConnConsensus
	queryConn     *appConnQuery

	clientCreator ClientCreator
}

// Make all necessary abci connections to the application
func NewMultiAppConn(clientCreator ClientCreator, handshaker Handshaker,
	logger log.Logger) *multiAppConn {

	multiAppConn := &multiAppConn{
		handshaker:    handshaker,
		clientCreator: clientCreator,
	}
	multiAppConn.BaseService = *cmn.NewBaseService(logger, "multiAppConn", multiAppConn)
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
	querycli, err := app.clientCreator.NewABCIClient(
		app.Logger.With("module", "abci-client", "connection", "query"))
	if err != nil {
		return errors.Wrap(err, "Error creating ABCI client (query connection)")
	}
	if _, err := querycli.Start(); err != nil {
		return errors.Wrap(err, "Error starting ABCI client (query connection)")
	}
	app.queryConn = NewAppConnQuery(querycli)

	// mempool connection
	memcli, err := app.clientCreator.NewABCIClient(
		app.Logger.With("module", "abci-client", "connection", "mempool"))
	if err != nil {
		return errors.Wrap(err, "Error creating ABCI client (mempool connection)")
	}
	if _, err := memcli.Start(); err != nil {
		return errors.Wrap(err, "Error starting ABCI client (mempool connection)")
	}
	app.mempoolConn = NewAppConnMempool(memcli)

	// consensus connection
	concli, err := app.clientCreator.NewABCIClient(
		app.Logger.With("module", "abci-client", "connection", "consensus"))
	if err != nil {
		return errors.Wrap(err, "Error creating ABCI client (consensus connection)")
	}
	if _, err := concli.Start(); err != nil {
		return errors.Wrap(err, "Error starting ABCI client (consensus connection)")
	}
	app.consensusConn = NewAppConnConsensus(concli)

	// ensure app is synced to the latest state
	if app.handshaker != nil {
		return app.handshaker.Handshake(app)
	}

	return nil
}
