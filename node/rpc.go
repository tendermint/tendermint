package node

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/rs/cors"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/strings"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	grpccore "github.com/tendermint/tendermint/rpc/grpc"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/types"
)

func startRPCServer(rpcConfig *cfg.RPCConfig, env *rpccore.Environment, logger log.Logger) (net.Listener, error) {
	// we expose a simplified api over grpc for convenience to app devs
	listener, err := rpcserver.Listen(rpcConfig.GRPCListenAddress, rpcConfig.GRPCMaxOpenConnections)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := grpccore.StartGRPCServer(env, listener); err != nil {
			logger.Error("Error starting gRPC server", "err", err)
		}
	}()
	return listener, nil
}

func startRPCServers(rpcConfig *cfg.RPCConfig, logger log.Logger, routes rpccore.RoutesMap, eventBus types.EventBusSubscriber) ([]net.Listener, error) {
	config := rpcserver.DefaultConfig()
	config.MaxBodyBytes = rpcConfig.MaxBodyBytes
	config.MaxHeaderBytes = rpcConfig.MaxHeaderBytes
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	if config.WriteTimeout <= rpcConfig.TimeoutBroadcastTxCommit {
		config.WriteTimeout = rpcConfig.TimeoutBroadcastTxCommit + 1*time.Second
	}

	// we may expose the rpc over both a unix and tcp socket
	listeners, err := listenersFromRPCConfig(rpcConfig)
	if err != nil {
		return nil, err
	}
	for _, listener := range listeners {
		mux := http.NewServeMux()
		registerWebsocketHandler(rpcConfig, mux, routes, logger, eventBus)
		rpcserver.RegisterRPCFuncs(mux, routes, logger)
		listenerAddr := listener.Addr().String()

		var rootHandler http.Handler = mux
		if rpcConfig.IsCorsEnabled() {
			rootHandler = addCORSHandler(rpcConfig, mux)
		}
		if rpcConfig.IsTLSEnabled() {
			go func() {
				keyFile := rpcConfig.KeyFile()
				certFile := rpcConfig.CertFile()
				logger.Info("RPC HTTPS server starting", "address", listenerAddr,
					"certfile", certFile, "keyfile", keyFile)
				err := rpcserver.ServeTLS(listener, rootHandler, keyFile, certFile, logger, config)
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("RPC HTTPS server stopped with error", "address", listener, "err", err)
					return
				}
				logger.Info("RPC HTTPS server stopped", "address", listenerAddr)
			}()
		} else {
			go func() {
				logger.Info("RPC HTTPS server starting", "address", listenerAddr)

				err := rpcserver.Serve(listener, rootHandler, logger, config)
				if !errors.Is(err, net.ErrClosed) {
					logger.Error("RPC HTTP server stopped with error", "address", listener, "err", err)
					return
				}
				logger.Info("RPC HTTP server stopped", "address", listenerAddr)
			}()
		}
	}

	return listeners, nil
}

func listenersFromRPCConfig(rpcConfig *cfg.RPCConfig) ([]net.Listener, error) {
	listenAddrs := strings.SplitAndTrimEmpty(rpcConfig.ListenAddress, ",", " ")
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		listener, err := rpcserver.Listen(listenAddr, rpcConfig.MaxOpenConnections)
		if err != nil {
			// close any listeners opened before returning
			for _, l := range listeners {
				if l != nil {
					l.Close()
				}
			}
			return nil, err
		}
		listeners[i] = listener
	}
	return listeners, nil
}

func registerWebsocketHandler(rpcConfig *cfg.RPCConfig,
	mux *http.ServeMux,
	routes rpccore.RoutesMap,
	logger log.Logger,
	eventBus types.EventBusSubscriber) {
	wmLogger := logger.With("protocol", "websocket")

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
}

func addCORSHandler(rpcConfig *cfg.RPCConfig, h http.Handler) http.Handler {
	if rpcConfig.IsCorsEnabled() {
		corsMiddleware := cors.New(cors.Options{
			AllowedOrigins: rpcConfig.CORSAllowedOrigins,
			AllowedMethods: rpcConfig.CORSAllowedMethods,
			AllowedHeaders: rpcConfig.CORSAllowedHeaders,
		})
		h = corsMiddleware.Handler(h)
	}
	return h
}
