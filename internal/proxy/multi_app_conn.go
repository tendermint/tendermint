package proxy

import (
	"context"
	"os"
	"syscall"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

// New creates a proxy application interface.
func New(clientCreator abciclient.Creator, logger log.Logger, metrics *Metrics) abciclient.Client {
	multiAppConn := &proxyConn{
		logger:        logger,
		metrics:       metrics,
		clientCreator: clientCreator,
	}
	multiAppConn.BaseService = *service.NewBaseService(logger, "multiAppConn", multiAppConn)
	return multiAppConn
}

// proxyConn implements provides the application connection.
type proxyConn struct {
	service.BaseService
	abciclient.Client

	logger log.Logger

	metrics *Metrics

	clientCreator abciclient.Creator
}

func (app *proxyConn) OnStop()                         { tryCallStop(app.Client) }
func (app *proxyConn) IsRunning() bool                 { return app.Client.IsRunning() }
func (app *proxyConn) Start(ctx context.Context) error { return app.BaseService.Start(ctx) }
func (app *proxyConn) Wait()                           { app.BaseService.Wait() }

func tryCallStop(client abciclient.Client) {
	if client == nil {
		return
	}
	switch c := client.(type) {
	case interface{ Stop() }:
		c.Stop()
	case *proxyClient:
		tryCallStop(c.Client)
	}
}

func (app *proxyConn) OnStart(ctx context.Context) error {
	var err error
	defer func() {
		if err != nil {
			tryCallStop(app.Client)
		}
	}()

	var client abciclient.Client
	client, err = app.clientCreator(app.logger)
	if err != nil {
		return err
	}

	app.Client = newProxyClient(client, app.metrics)
	// Kill Tendermint if the ABCI application crashes.
	go func() {
		if !app.Client.IsRunning() {
			return
		}
		app.Client.Wait()
		if ctx.Err() != nil {
			return
		}

		if err := app.Client.Error(); err != nil {
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

func kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	return p.Signal(syscall.SIGTERM)
}
