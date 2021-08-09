package rpc

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/rpc/core"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	"github.com/tendermint/tendermint/rpc/jsonrpc/server"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

type Server struct {
	Addr    string // TCP address to listen on, ":http" if empty
	Handler http.Handler
	Logger  log.Logger
	Config  *config.RPCConfig
}

func Routes(store state.Store, blockStore state.BlockStore, eventSinks []indexer.EventSink) rpccore.RoutesMap {
	env := &core.Environment{
		EventSinks: eventSinks,
		StateStore: store,
		BlockStore: blockStore,
	}
	return rpccore.RoutesMap{
		"blockchain":       rpcserver.NewRPCFunc(env.BlockchainInfo, "minHeight,maxHeight", true),
		"consensus_params": rpcserver.NewRPCFunc(env.ConsensusParams, "height", true),
		"block":            rpcserver.NewRPCFunc(env.Block, "height", true),
		"block_by_hash":    rpcserver.NewRPCFunc(env.BlockByHash, "hash", true),
		"block_results":    rpcserver.NewRPCFunc(env.BlockResults, "height", true),
		"commit":           rpcserver.NewRPCFunc(env.Commit, "height", true),
		"validators":       rpcserver.NewRPCFunc(env.Validators, "height,page,per_page", true),
		"tx":               rpcserver.NewRPCFunc(env.Tx, "hash,prove", true),
		"tx_search":        rpcserver.NewRPCFunc(env.TxSearch, "query,prove,page,per_page,order_by", false),
		"block_search":     rpcserver.NewRPCFunc(env.BlockSearch, "query,page,per_page,order_by", false),
	}
}

func Handler(rpcConfig *config.RPCConfig, routes rpccore.RoutesMap, logger log.Logger) http.Handler {
	mux := http.NewServeMux()
	wmLogger := logger.With("protocol", "websocket")

	var eventBus types.EventBusSubscriber

	websocketDisconnectFn := func(remoteAddr string) {
		err := eventBus.UnsubscribeAll(context.Background(), remoteAddr)
		if err != nil && err != tmpubsub.ErrSubscriptionNotFound {
			wmLogger.Error("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
		}
	}
	wm := rpcserver.NewWebsocketManager(routes,
		rpcserver.OnDisconnect(websocketDisconnectFn),
		rpcserver.ReadLimit(rpcConfig.MaxBodyBytes))
	wm.SetLogger(wmLogger)
	mux.HandleFunc("/websocket", wm.WebsocketHandler)

	rpcserver.RegisterRPCFuncs(mux, routes, logger)
	var rootHandler http.Handler = mux
	if rpcConfig.IsCorsEnabled() {
		rootHandler = addCORSHandler(rpcConfig, mux)
	}
	return rootHandler
}

func addCORSHandler(rpcConfig *config.RPCConfig, h http.Handler) http.Handler {
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: rpcConfig.CORSAllowedOrigins,
		AllowedMethods: rpcConfig.CORSAllowedMethods,
		AllowedHeaders: rpcConfig.CORSAllowedHeaders,
	})
	h = corsMiddleware.Handler(h)
	return h
}

func (r *Server) ListenAndServe(ctx context.Context) error {
	listener, err := rpcserver.Listen(r.Addr, r.Config.MaxOpenConnections)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	return rpcserver.Serve(listener, r.Handler, r.Logger, serverRPCConfig(r.Config))
}

func (r *Server) ListenAndServeTLS(ctx context.Context, certFile, keyFile string) error {
	listener, err := rpcserver.Listen(r.Addr, r.Config.MaxOpenConnections)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	return rpcserver.ServeTLS(listener, r.Handler, certFile, keyFile, r.Logger, serverRPCConfig(r.Config))
}

func serverRPCConfig(r *config.RPCConfig) *server.Config {
	cfg := rpcserver.DefaultConfig()
	cfg.MaxBodyBytes = r.MaxBodyBytes
	cfg.MaxHeaderBytes = r.MaxHeaderBytes
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	if cfg.WriteTimeout <= r.TimeoutBroadcastTxCommit {
		cfg.WriteTimeout = r.TimeoutBroadcastTxCommit + 1*time.Second
	}
	return cfg
}
