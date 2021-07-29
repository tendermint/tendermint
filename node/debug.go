package node

import (
	"net"

	"github.com/tendermint/tendermint/config"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

// Debug is a type useful for debugging tendermint problems.
// Tendermint nodes will shutdown if a divergent hash is detected. Once in this
// state, they will not start up again. Debug runs just an RPC server on the
// tendermint data stores without running any other components. This way a user
// can query the RPC server to diagnose the issue that caused a crash to begin with.
type Debug struct {
	service.BaseService

	blockStore sm.BlockStore
	stateStore sm.Store

	rpcConfig *cfg.RPCConfig
	listeners []net.Listener
}

func NewDebugFromConfig(cfg *config.Config) (*Debug, error) {
	blockStoreDB, err := config.DefaultDBProvider(&config.DBContext{ID: _blockStoreID, Config: cfg})
	if err != nil {
		return nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)
	stateDB, err := config.DefaultDBProvider(&config.DBContext{ID: _stateStoreID, Config: cfg})
	stateStore := sm.NewStore(stateDB)

	return NewDebug(cfg.RPC, blockStore, stateStore), nil
}

func NewDebug(rpcConfig *cfg.RPCConfig, blockStore sm.BlockStore, stateStore sm.Store) *Debug {
	return &Debug{
		blockStore: blockStore,
		stateStore: stateStore,
		rpcConfig:  rpcConfig,
	}
}

func NewDefaultDebug() (*Debug, error) {
	cfg := config.Config{
		BaseConfig: config.DefaultBaseConfig(),
		RPC:        config.DefaultRPCConfig(),
	}
	return NewDebugFromConfig(&cfg)
}

func (debug *Debug) OnStart() error {
	rpcCoreEnv := rpccore.Environment{
		StateStore: debug.stateStore,
		BlockStore: debug.blockStore,
	}
	routes := rpcCoreEnv.InfoRoutes()
	l := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	listeners, err := startHTTPRPCServer(debug.rpcConfig, l, routes, types.NopEventBus{})
	if err != nil {
		return err
	}
	debug.listeners = listeners
	return nil
}

func (debug *Debug) OnStop() {
	for _, listener := range debug.listeners {
		listener.Close()
	}
}
