package inspect

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/inspect/rpc"
	rpccore "github.com/tendermint/tendermint/internal/rpc/core"
	"github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	tmstrings "github.com/tendermint/tendermint/libs/strings"
	"github.com/tendermint/tendermint/types"

	"golang.org/x/sync/errgroup"
)

// Inspector manages an RPC service that exports methods to debug a failed node.
// After a node shuts down due to a consensus failure, it will no longer start
// up its state cannot easily be inspected. An Inspector value provides a similar interface
// to the node, using the underlying Tendermint data stores, without bringing up
// any other components. A caller can query the Inspector service to inspect the
// persisted state and debug the failure.
type Inspector struct {
	routes rpccore.RoutesMap

	config *config.RPCConfig

	indexerService *indexer.Service
	eventBus       *eventbus.EventBus
	logger         log.Logger
}

// New returns an Inspector that serves RPC on the specified BlockStore and StateStore.
// The Inspector type does not modify the state or block stores.
// The sinks are used to enable block and transaction querying via the RPC server.
// The caller is responsible for starting and stopping the Inspector service.
func New(cfg *config.RPCConfig, bs state.BlockStore, ss state.Store, es []indexer.EventSink, logger log.Logger) *Inspector {
	eb := eventbus.NewDefault(logger.With("module", "events"))

	return &Inspector{
		routes:   rpc.Routes(*cfg, ss, bs, es, logger),
		config:   cfg,
		logger:   logger,
		eventBus: eb,
		indexerService: indexer.NewService(indexer.ServiceArgs{
			Sinks:    es,
			EventBus: eb,
			Logger:   logger.With("module", "txindex"),
		}),
	}
}

// NewFromConfig constructs an Inspector using the values defined in the passed in config.
func NewFromConfig(logger log.Logger, cfg *config.Config) (*Inspector, error) {
	bsDB, err := config.DefaultDBProvider(&config.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		return nil, err
	}
	bs := store.NewBlockStore(bsDB)
	sDB, err := config.DefaultDBProvider(&config.DBContext{ID: "state", Config: cfg})
	if err != nil {
		return nil, err
	}
	genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
	if err != nil {
		return nil, err
	}
	sinks, err := sink.EventSinksFromConfig(cfg, config.DefaultDBProvider, genDoc.ChainID)
	if err != nil {
		return nil, err
	}
	ss := state.NewStore(sDB)
	return New(cfg.RPC, bs, ss, sinks, logger), nil
}

// Run starts the Inspector servers and blocks until the servers shut down. The passed
// in context is used to control the lifecycle of the servers.
func (ins *Inspector) Run(ctx context.Context) error {
	err := ins.eventBus.Start(ctx)
	if err != nil {
		return fmt.Errorf("error starting event bus: %s", err)
	}
	defer ins.eventBus.Wait()

	err = ins.indexerService.Start(ctx)
	if err != nil {
		return fmt.Errorf("error starting indexer service: %s", err)
	}
	defer ins.indexerService.Wait()

	return startRPCServers(ctx, ins.config, ins.logger, ins.routes)
}

func startRPCServers(ctx context.Context, cfg *config.RPCConfig, logger log.Logger, routes rpccore.RoutesMap) error {
	g, tctx := errgroup.WithContext(ctx)
	listenAddrs := tmstrings.SplitAndTrimEmpty(cfg.ListenAddress, ",", " ")
	rh := rpc.Handler(cfg, routes, logger)
	for _, listenerAddr := range listenAddrs {
		server := rpc.Server{
			Logger:  logger,
			Config:  cfg,
			Handler: rh,
			Addr:    listenerAddr,
		}
		if cfg.IsTLSEnabled() {
			keyFile := cfg.KeyFile()
			certFile := cfg.CertFile()
			listenerAddr := listenerAddr
			g.Go(func() error {
				logger.Info("RPC HTTPS server starting", "address", listenerAddr,
					"certfile", certFile, "keyfile", keyFile)
				err := server.ListenAndServeTLS(tctx, certFile, keyFile)
				if !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				logger.Info("RPC HTTPS server stopped", "address", listenerAddr)
				return nil
			})
		} else {
			listenerAddr := listenerAddr
			g.Go(func() error {
				logger.Info("RPC HTTP server starting", "address", listenerAddr)
				err := server.ListenAndServe(tctx)
				if !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				logger.Info("RPC HTTP server stopped", "address", listenerAddr)
				return nil
			})
		}
	}
	return g.Wait()
}
