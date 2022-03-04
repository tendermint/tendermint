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
func New(client abciclient.Client, logger log.Logger, metrics *Metrics) abciclient.Client {
	conn := &proxyClient{
		logger:  logger,
		metrics: metrics,
		client:  client,
	}
	conn.BaseService = *service.NewBaseService(logger, "proxyClient", conn)
	return conn
}

// proxyClient implements provides the application connection.
type proxyClient struct {
	service.BaseService
	logger log.Logger

	client  abciclient.Client
	metrics *Metrics
}

func (app *proxyClient) OnStop()      { tryCallStop(app.client) }
func (app *proxyClient) Error() error { return app.client.Error() }

func tryCallStop(client abciclient.Client) {
	switch c := client.(type) {
	case nil:
		return
	case interface{ Stop() }:
		c.Stop()
	}
}

func (app *proxyClient) OnStart(ctx context.Context) error {
	var err error
	defer func() {
		if err != nil {
			tryCallStop(app.client)
		}
	}()

	// Kill Tendermint if the ABCI application crashes.
	go func() {
		if !app.client.IsRunning() {
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

	return app.client.Start(ctx)
}

func kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	return p.Signal(syscall.SIGTERM)
}
