package inspect

import (
	"context"
	"errors"
	"net"
	"sync"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/inspect/rpc"
	inspect_rpc "github.com/tendermint/tendermint/inspect/rpc"
	"github.com/tendermint/tendermint/libs/log"
	tmstrings "github.com/tendermint/tendermint/libs/strings"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/indexer/sink"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

// Inspect manages an RPC service that exports methods to debug a failed node.
// After a node shuts down due to a consensus failure, it will no longer start
// up its state cannot easily be inspected. An Inspect value provides a similar interface
// to the node, using the underlying Tendermint data stores, without bringing up
// any other components. A caller can query the Inspect service to inspect the
// persisted state and debug the failure.
type Inspect struct {
	routes rpccore.RoutesMap

	rpcConfig *cfg.RPCConfig

	indexerService *indexer.Service
	eventBus       *types.EventBus
	logger         log.Logger
}

// New constructs a new Inspect from the passed in parameters.
func New(rpcConfig *cfg.RPCConfig, blockStore sm.BlockStore, stateStore sm.Store, eventSinks []indexer.EventSink, logger log.Logger) *Inspect {
	routes := inspect_rpc.Routes(stateStore, blockStore, eventSinks)
	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.With("module", "events"))
	indexerService := indexer.NewIndexerService(eventSinks, eventBus)
	indexerService.SetLogger(logger.With("module", "txindex"))
	return &Inspect{
		routes:         routes,
		rpcConfig:      rpcConfig,
		logger:         logger,
		eventBus:       eventBus,
		indexerService: indexerService,
	}
}

// NewFromConfig constructs an Inspect using the values defined in the passed in config.
func NewFromConfig(config *cfg.Config) (*Inspect, error) {
	blockStoreDB, err := cfg.DefaultDBProvider(&cfg.DBContext{ID: "blockstore", Config: config})
	if err != nil {
		return nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)
	stateDB, err := cfg.DefaultDBProvider(&cfg.DBContext{ID: "statestore", Config: config})
	if err != nil {
		return nil, err
	}
	genDoc, err := types.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		return nil, err
	}
	sinks, err := sink.EventSinksFromConfig(config, cfg.DefaultDBProvider, genDoc.ChainID)
	if err != nil {
		return nil, err
	}
	l := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	stateStore := sm.NewStore(stateDB)
	return New(config.RPC, blockStore, stateStore, sinks, l), nil
}

// NewDefault constructs a new Inspect using the default values.
func NewDefault() (*Inspect, error) {
	config := cfg.Config{
		BaseConfig: cfg.DefaultBaseConfig(),
		RPC:        cfg.DefaultRPCConfig(),
		TxIndex:    cfg.DefaultTxIndexConfig(),
	}
	return NewFromConfig(&config)
}

// Run starts the Inspect servers and blocks until the servers shut down. The passed
// in context is used to control the lifecycle of the servers.
func (inspect *Inspect) Run(ctx context.Context) error {
	err := inspect.eventBus.Start()
	if err != nil {
		return err
	}
	defer inspect.eventBus.Stop()

	err = inspect.indexerService.Start()
	if err != nil {
		return err
	}
	defer inspect.indexerService.Stop()
	return startRPCServers(ctx, inspect.rpcConfig, inspect.logger, inspect.routes)
}

func startRPCServers(ctx context.Context, rpcConfig *cfg.RPCConfig, logger log.Logger, routes rpccore.RoutesMap) error {
	wg := &sync.WaitGroup{}
	listenAddrs := tmstrings.SplitAndTrimEmpty(rpcConfig.ListenAddress, ",", " ")
	rootHandler := inspect_rpc.Handler(rpcConfig, routes, logger)
	errChan := make(chan error)
	for _, listenerAddr := range listenAddrs {
		server := rpc.Server{
			Logger:  logger,
			Config:  rpcConfig,
			Handler: rootHandler,
			Addr:    listenerAddr,
		}
		if rpcConfig.IsTLSEnabled() {
			keyFile := rpcConfig.KeyFile()
			certFile := rpcConfig.CertFile()
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Info("RPC HTTPS server starting", "address", listenerAddr,
					"certfile", certFile, "keyfile", keyFile)
				err := server.ListenAndServeTLS(ctx, certFile, keyFile)
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("RPC HTTPS server stopped with error", "address", listenerAddr, "err", err)
					errChan <- err
					return
				}
				logger.Info("RPC HTTPS server stopped", "address", listenerAddr)
			}()
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Info("RPC HTTP server starting", "address", listenerAddr)
				err := server.ListenAndServe(ctx)
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("RPC HTTP server stopped with error", "address", listenerAddr, "err", err)
					errChan <- err
					return
				}
				logger.Info("RPC HTTP server stopped", "address", listenerAddr)
			}()
		}
	}
	select {
	case <-chanFromWG(wg):
		return nil
	case err := <-errChan:
		return err
	}
}

func chanFromWG(wg *sync.WaitGroup) chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}
