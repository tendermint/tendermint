package inspect

import (
	"context"
	"errors"
	"net"
	"os"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/inspect/rpc"
	"github.com/tendermint/tendermint/libs/log"
	tmstrings "github.com/tendermint/tendermint/libs/strings"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/indexer/block"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"

	"golang.org/x/sync/errgroup"
)

var (
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
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

	logger log.Logger

	// References to the state store and block store are maintained to enable
	// the Inspector to safely close them on shutdown.
	ss state.Store
	bs state.BlockStore
}

// New returns an Inspector that serves RPC on the specified BlockStore and StateStore.
// The Inspector type does not modify the state or block stores.
// The sinks are used to enable block and transaction querying via the RPC server.
// The caller is responsible for starting and stopping the Inspector service.
//
//nolint:lll
func New(cfg *config.RPCConfig, bs state.BlockStore, ss state.Store, txidx txindex.TxIndexer, blkidx indexer.BlockIndexer, lg log.Logger) *Inspector {
	routes := rpc.Routes(*cfg, ss, bs, txidx, blkidx, logger)
	eb := types.NewEventBus()
	eb.SetLogger(logger.With("module", "events"))
	return &Inspector{
		routes: routes,
		config: cfg,
		logger: logger,
		ss:     ss,
		bs:     bs,
	}
}

// NewFromConfig constructs an Inspector using the values defined in the passed in config.
func NewFromConfig(cfg *config.Config) (*Inspector, error) {
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
	txidx, blkidx, err := block.IndexerFromConfig(cfg, config.DefaultDBProvider, genDoc.ChainID)
	if err != nil {
		return nil, err
	}
	lg := logger.With("module", "inspect")
	ss := state.NewStore(sDB, state.StoreOptions{})
	return New(cfg.RPC, bs, ss, txidx, blkidx, lg), nil
}

// Run starts the Inspector servers and blocks until the servers shut down. The passed
// in context is used to control the lifecycle of the servers.
func (ins *Inspector) Run(ctx context.Context) error {
	defer ins.bs.Close()
	defer ins.ss.Close()

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
				if !errors.Is(err, net.ErrClosed) {
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
				if !errors.Is(err, net.ErrClosed) {
					return err
				}
				logger.Info("RPC HTTP server stopped", "address", listenerAddr)
				return nil
			})
		}
	}
	return g.Wait()
}
