package proxy

import (
	"context"
	"fmt"
	"os"
	"syscall"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

const (
	connConsensus = "consensus"
	connMempool   = "mempool"
	connQuery     = "query"
	connSnapshot  = "snapshot"
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

	consensusConnClient stoppableClient
	mempoolConnClient   stoppableClient
	queryConnClient     stoppableClient
	snapshotConnClient  stoppableClient

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
			app.stopAllClients()
		}
	}()

	app.queryConnClient, err = app.abciClientFor(ctx, connQuery)
	if err != nil {
		return err
	}
	app.queryConn = NewAppConnQuery(app.queryConnClient, app.metrics)

	app.snapshotConnClient, err = app.abciClientFor(ctx, connSnapshot)
	if err != nil {
		return err
	}
	app.snapshotConn = NewAppConnSnapshot(app.snapshotConnClient, app.metrics)

	app.mempoolConnClient, err = app.abciClientFor(ctx, connMempool)
	if err != nil {
		return err
	}
	app.mempoolConn = NewAppConnMempool(app.mempoolConnClient, app.metrics)

	app.consensusConnClient, err = app.abciClientFor(ctx, connConsensus)
	if err != nil {
		return err
	}
	app.consensusConn = NewAppConnConsensus(app.consensusConnClient, app.metrics)

	// Kill Tendermint if the ABCI application crashes.
	app.startWatchersForClientErrorToKillTendermint(ctx)

	return nil
}

func (app *multiAppConn) OnStop() { app.stopAllClients() }

func (app *multiAppConn) startWatchersForClientErrorToKillTendermint(ctx context.Context) {
	// this function starts a number of threads (per abci client)
	// that will SIGTERM's our own PID if any of the ABCI clients
	// exit/return early. If the context is canceled then these
	// functions will not kill tendermint.

	killFn := func(conn string, err error, logger log.Logger) {
		logger.Error(
			fmt.Sprintf("%s connection terminated. Did the application crash? Please restart tendermint", conn),
			"err", err)
		if killErr := kill(); killErr != nil {
			logger.Error("Failed to kill this process - please do so manually", "err", killErr)
		}
	}

	for _, client := range []struct {
		connClient stoppableClient
		name       string
	}{
		{
			connClient: app.consensusConnClient,
			name:       connConsensus,
		},
		{
			connClient: app.mempoolConnClient,
			name:       connMempool,
		},
		{
			connClient: app.queryConnClient,
			name:       connQuery,
		},
		{
			connClient: app.snapshotConnClient,
			name:       connSnapshot,
		},
	} {
		go func(name string, client stoppableClient) {
			client.Wait()
			if ctx.Err() != nil {
				return
			}
			if err := client.Error(); err != nil {
				killFn(name, err, app.logger)
			}
		}(client.name, client.connClient)
	}
}

func (app *multiAppConn) stopAllClients() {
	for _, client := range []stoppableClient{
		app.consensusConnClient,
		app.mempoolConnClient,
		app.queryConnClient,
		app.snapshotConnClient,
	} {
		if client != nil {
			client.Stop()
		}
	}
}

func (app *multiAppConn) abciClientFor(ctx context.Context, conn string) (stoppableClient, error) {
	c, err := app.clientCreator(app.logger.With(
		"module", "abci-client",
		"connection", conn))
	if err != nil {
		return nil, fmt.Errorf("error creating ABCI client (%s connection): %w", conn, err)
	}

	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("error starting ABCI client (%s connection): %w", conn, err)
	}

	client, ok := c.(stoppableClient)
	if !ok {
		return nil, fmt.Errorf("%T is not a stoppable client", c)
	}

	return client, nil
}

func kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	return p.Signal(syscall.SIGTERM)
}
