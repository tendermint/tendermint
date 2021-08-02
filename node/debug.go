package node

import (
	"net"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

// Debug manages an RPC service that exports methods to debug a failed node.
// After a node shuts down due to a consensus failure,, it will no longer start
// up and cannot easily be inspected. A Debug value provides a similar interface
// to the node, using the underlying Tendermint data stores, without bringing up
// any other components. A caller can query the Debug service to inspect the
// persisted state and debug the failure.
type Debug struct {
	service.BaseService

	routes rpccore.RoutesMap

	rpcConfig *cfg.RPCConfig
	listeners []net.Listener
}

func NewDebugFromConfig(config *cfg.Config) (*Debug, error) {
	blockStoreDB, err := cfg.DefaultDBProvider(&cfg.DBContext{ID: _blockStoreID, Config: config})
	if err != nil {
		return nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)
	stateDB, err := cfg.DefaultDBProvider(&cfg.DBContext{ID: _stateStoreID, Config: config})
	if err != nil {
		return nil, err
	}
	genDocFunc := defaultGenesisDocProviderFunc(config)
	genDoc, err := genDocFunc()
	if err != nil {
		return nil, err
	}
	sinks, err := indexerSinksFromConfig(config, cfg.DefaultDBProvider, genDoc.ChainID)
	if err != nil {
		return nil, err
	}
	stateStore := sm.NewStore(stateDB)
	return NewDebug(config.RPC, blockStore, stateStore, sinks), nil
}

func NewDebug(rpcConfig *cfg.RPCConfig, blockStore sm.BlockStore, stateStore sm.Store, eventSinks []indexer.EventSink) *Debug {
	routes := rpccore.DebugRoutes(stateStore, blockStore, eventSinks)
	return &Debug{
		routes:    routes,
		rpcConfig: rpcConfig,
	}
}

func NewDefaultDebug() (*Debug, error) {
	config := cfg.Config{
		BaseConfig: cfg.DefaultBaseConfig(),
		RPC:        cfg.DefaultRPCConfig(),
		TxIndex:    cfg.DefaultTxIndexConfig(),
	}
	return NewDebugFromConfig(&config)
}

func (debug *Debug) OnStart() error {
	l := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	listeners, err := startRPCServers(debug.rpcConfig, l, debug.routes, &types.NopEventBus{})
	if err != nil {
		return err
	}
	debug.listeners = listeners
	return nil
}

func (debug *Debug) OnStop() {
	for i := len(debug.listeners) - 1; i >= 0; i-- {
		err := debug.listeners[i].Close()
		if err != nil {
			debug.Logger.Error("Error stopping debug rpc listener", "err", err)
		}
	}
}
