package rpc

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/cors"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/pubsub"
	"github.com/tendermint/tendermint/internal/rpc/core"
	"github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// Server defines parameters for running an Inspector rpc server.
type Server struct {
	Addr    string // TCP address to listen on, ":http" if empty
	Handler http.Handler
	Logger  log.Logger
	Config  *config.RPCConfig
}

type eventBusUnsubscriber interface {
	UnsubscribeAll(ctx context.Context, subscriber string) error
}

// Routes returns the set of routes used by the Inspector server.
func Routes(cfg config.RPCConfig, s state.Store, bs state.BlockStore, es []indexer.EventSink, logger log.Logger) core.RoutesMap {
	env := &core.Environment{
		Config:     cfg,
		EventSinks: es,
		StateStore: s,
		BlockStore: bs,
		Logger:     logger,
	}
	return core.RoutesMap{
		"blockchain":       server.NewRPCFunc(env.BlockchainInfo),
		"consensus_params": server.NewRPCFunc(env.ConsensusParams),
		"block":            server.NewRPCFunc(env.Block),
		"block_by_hash":    server.NewRPCFunc(env.BlockByHash),
		"block_results":    server.NewRPCFunc(env.BlockResults),
		"commit":           server.NewRPCFunc(env.Commit),
		"validators":       server.NewRPCFunc(env.Validators),
		"tx":               server.NewRPCFunc(env.Tx),
		"tx_search":        server.NewRPCFunc(env.TxSearch),
		"block_search":     server.NewRPCFunc(env.BlockSearch),
	}
}

// Handler returns the http.Handler configured for use with an Inspector server. Handler
// registers the routes on the http.Handler and also registers the websocket handler
// and the CORS handler if specified by the configuration options.
func Handler(rpcConfig *config.RPCConfig, routes core.RoutesMap, logger log.Logger) http.Handler {
	mux := http.NewServeMux()
	wmLogger := logger.With("protocol", "websocket")

	var eventBus eventBusUnsubscriber

	websocketDisconnectFn := func(remoteAddr string) {
		err := eventBus.UnsubscribeAll(context.Background(), remoteAddr)
		if err != nil && err != pubsub.ErrSubscriptionNotFound {
			wmLogger.Error("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
		}
	}
	wm := server.NewWebsocketManager(logger, routes,
		server.OnDisconnect(websocketDisconnectFn),
		server.ReadLimit(rpcConfig.MaxBodyBytes))
	mux.HandleFunc("/websocket", wm.WebsocketHandler)

	server.RegisterRPCFuncs(mux, routes, logger)
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

// ListenAndServe listens on the address specified in srv.Addr and handles any
// incoming requests over HTTP using the Inspector rpc handler specified on the server.
func (srv *Server) ListenAndServe(ctx context.Context) error {
	listener, err := server.Listen(srv.Addr, srv.Config.MaxOpenConnections)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	return server.Serve(ctx, listener, srv.Handler, srv.Logger, serverRPCConfig(srv.Config))
}

// ListenAndServeTLS listens on the address specified in srv.Addr. ListenAndServeTLS handles
// incoming requests over HTTPS using the Inspector rpc handler specified on the server.
func (srv *Server) ListenAndServeTLS(ctx context.Context, certFile, keyFile string) error {
	listener, err := server.Listen(srv.Addr, srv.Config.MaxOpenConnections)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	return server.ServeTLS(ctx, listener, srv.Handler, certFile, keyFile, srv.Logger, serverRPCConfig(srv.Config))
}

func serverRPCConfig(r *config.RPCConfig) *server.Config {
	cfg := server.DefaultConfig()
	cfg.MaxBodyBytes = r.MaxBodyBytes
	cfg.MaxHeaderBytes = r.MaxHeaderBytes
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	// Note we don't need to adjust anything if the timeout is already unlimited.
	if cfg.WriteTimeout > 0 && cfg.WriteTimeout <= r.TimeoutBroadcastTxCommit {
		cfg.WriteTimeout = r.TimeoutBroadcastTxCommit + 1*time.Second
	}
	return cfg
}
