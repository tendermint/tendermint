package inspect

import (
	"context"
	"errors"
	"net"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/inspect/rpc"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/pubsub"
	tmstrings "github.com/tendermint/tendermint/libs/strings"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/indexer/sink"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"

	"golang.org/x/sync/errgroup"
)

// Inspect manages an RPC service that exports methods to debug a failed node.
// After a node shuts down due to a consensus failure, it will no longer start
// up its state cannot easily be inspected. An Inspect value provides a similar interface
// to the node, using the underlying Tendermint data stores, without bringing up
// any other components. A caller can query the Inspect service to inspect the
// persisted state and debug the failure.
type Inspect struct {
	routes rpccore.RoutesMap

	config *config.RPCConfig

	indexerService *indexer.Service
	eventBus       *types.EventBus
	logger         log.Logger
}

// New returns an Inspect that serves RPC on the given BlockStore and StateStore.
///
//nolint:lll
func New(cfg *config.RPCConfig, bs state.BlockStore, ss state.Store, es []indexer.EventSink, logger log.Logger) *Inspect {
	routes := rpc.Routes(ss, bs, es)
	eb := types.NewEventBus()
	eb.SetLogger(logger.With("module", "events"))
	is := indexer.NewIndexerService(es, eb)
	is.SetLogger(logger.With("module", "txindex"))
	return &Inspect{
		routes:         routes,
		config:         cfg,
		logger:         logger,
		eventBus:       eb,
		indexerService: is,
	}
}

// NewFromConfig constructs an Inspect using the values defined in the passed in config.
func NewFromConfig(cfg *config.Config) (*Inspect, error) {
	bsDB, err := config.DefaultDBProvider(&config.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		return nil, err
	}
	bs := store.NewBlockStore(bsDB)
	sDB, err := config.DefaultDBProvider(&config.DBContext{ID: "statestore", Config: cfg})
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
	logger := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	ss := state.NewStore(sDB)
	return New(cfg.RPC, bs, ss, sinks, logger), nil
}

// Run starts the Inspect servers and blocks until the servers shut down. The passed
// in context is used to control the lifecycle of the servers.
func (ins *Inspect) Run(ctx context.Context) error {
	err := ins.eventBus.Start()
	if err != nil {
		return err
	}
	defer func() {
		err := ins.eventBus.Stop()
		if err != nil {
			ins.logger.Error("event bus stopped with error", "err", err)
		}
	}()
	g, tctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if !errors.Is(ins.indexerService.Start(), pubsub.ErrUnsubscribed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		return startRPCServers(tctx, ins.config, ins.logger, ins.routes)
	})

	<-tctx.Done()
	err = ins.indexerService.Stop()
	if err != nil {
		ins.logger.Error("event bus stopped with error", "err", err)
	}
	return g.Wait()
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
			g.Go(func() error {
				logger.Info("RPC HTTPS server starting", "address", listenerAddr,
					"certfile", certFile, "keyfile", keyFile)
				err := server.ListenAndServeTLS(tctx, certFile, keyFile)
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("RPC HTTPS server stopped with error", "address", listenerAddr, "err", err)
					return err
				}
				logger.Info("RPC HTTPS server stopped", "address", listenerAddr)
				return nil
			})
		} else {
			g.Go(func() error {
				logger.Info("RPC HTTP server starting", "address", listenerAddr)
				err := server.ListenAndServe(tctx)
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("RPC HTTP server stopped with error", "address", listenerAddr, "err", err)
					return err
				}
				logger.Info("RPC HTTP server stopped", "address", listenerAddr)
				return nil
			})
		}
	}
	return g.Wait()
}
