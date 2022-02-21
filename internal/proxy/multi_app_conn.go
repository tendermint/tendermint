package proxy

import (
	"context"
	"os"
	"syscall"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/libs/log"
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
func NewAppConns(clientCreator abciclient.Creator, logger log.Logger, metrics *Metrics) AppConns {
	return NewMultiAppConn(clientCreator, logger, metrics)
}

// multiAppConn implements AppConns.
//
// A multiAppConn is made of a few appConns and manages their underlying abci
// clients.
// TODO: on app restart, clients must reboot together
type multiAppConn struct {
	service.BaseService
	logger log.Logger

	metrics       *Metrics
	consensusConn AppConnConsensus
	mempoolConn   AppConnMempool
	queryConn     AppConnQuery
	snapshotConn  AppConnSnapshot

	client stoppableClient

	clientCreator abciclient.Creator
}

// TODO: this is a totally internal and quasi permanent shim for
// clients. eventually we can have a single client and have some kind
// of reasonable lifecycle witout needing an explicit stop method.
type stoppableClient interface {
	abciclient.Client
	Stop()
}

// NewMultiAppConn makes all necessary abci connections to the application.
func NewMultiAppConn(clientCreator abciclient.Creator, logger log.Logger, metrics *Metrics) AppConns {
	multiAppConn := &multiAppConn{
		logger:        logger,
		metrics:       metrics,
		clientCreator: clientCreator,
	}
	multiAppConn.BaseService = *service.NewBaseService(logger, "multiAppConn", multiAppConn)
	return multiAppConn
}

func (app *multiAppConn) Mempool() AppConnMempool     { return app.mempoolConn }
func (app *multiAppConn) Consensus() AppConnConsensus { return app.consensusConn }
func (app *multiAppConn) Query() AppConnQuery         { return app.queryConn }
func (app *multiAppConn) Snapshot() AppConnSnapshot   { return app.snapshotConn }

func (app *multiAppConn) OnStart(ctx context.Context) error {
	var err error
	defer func() {
		if err != nil {
			app.client.Stop()
		}
	}()

	var client abciclient.Client
	client, err = app.clientCreator(app.logger)
	if err != nil {
		return err
	}

	app.queryConn = NewAppConnQuery(client, app.metrics)
	app.snapshotConn = NewAppConnSnapshot(client, app.metrics)
	app.mempoolConn = NewAppConnMempool(client, app.metrics)
	app.consensusConn = NewAppConnConsensus(client, app.metrics)

	app.client = client.(stoppableClient)

	// Kill Tendermint if the ABCI application crashes.
	go func() {
		if !client.IsRunning() {
			return
		}
		app.client.Wait()
		if ctx.Err() != nil {
			return
		}

		if err := app.client.Error(); err != nil {
			app.logger.Error("client connection terminated. Did the application crash? Please restart tendermint",
				"err", err)
			if killErr := kill(); killErr != nil {
				app.logger.Error("Failed to kill this process - please do so manually",
					"err", killErr)
			}
		}

	}()

	return client.Start(ctx)
}

func (app *multiAppConn) OnStop() { app.client.Stop() }

func kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	return p.Signal(syscall.SIGTERM)
}
