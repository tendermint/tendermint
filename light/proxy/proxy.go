package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"

	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	rpccore "github.com/tendermint/tendermint/internal/rpc/core"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// A Proxy defines parameters for running an HTTP server proxy.
type Proxy struct {
	Addr     string // TCP address to listen on, ":http" if empty
	Config   *rpcserver.Config
	Client   *lrpc.Client
	Logger   log.Logger
	Listener net.Listener
}

// NewProxy creates the struct used to run an HTTP server for serving light
// client rpc requests.
func NewProxy(
	lightClient *light.Client,
	listenAddr, providerAddr string,
	config *rpcserver.Config,
	logger log.Logger,
	opts ...lrpc.Option,
) (*Proxy, error) {
	rpcClient, err := rpchttp.NewWithTimeout(providerAddr, config.WriteTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client for %s: %w", providerAddr, err)
	}

	return &Proxy{
		Addr:   listenAddr,
		Config: config,
		Client: lrpc.NewClient(logger, rpcClient, lightClient, opts...),
		Logger: logger,
	}, nil
}

// ListenAndServe configures the rpcserver.WebsocketManager, sets up the RPC
// routes to proxy via Client, and starts up an HTTP server on the TCP network
// address p.Addr.
// See http#Server#ListenAndServe.
func (p *Proxy) ListenAndServe(ctx context.Context) error {
	listener, mux, err := p.listen(ctx)
	if err != nil {
		return err
	}
	p.Listener = listener

	return rpcserver.Serve(
		ctx,
		listener,
		mux,
		p.Logger,
		p.Config,
	)
}

// ListenAndServeTLS acts identically to ListenAndServe, except that it expects
// HTTPS connections.
// See http#Server#ListenAndServeTLS.
func (p *Proxy) ListenAndServeTLS(ctx context.Context, certFile, keyFile string) error {
	listener, mux, err := p.listen(ctx)
	if err != nil {
		return err
	}
	p.Listener = listener

	return rpcserver.ServeTLS(
		ctx,
		listener,
		mux,
		certFile,
		keyFile,
		p.Logger,
		p.Config,
	)
}

func (p *Proxy) listen(ctx context.Context) (net.Listener, *http.ServeMux, error) {
	mux := http.NewServeMux()

	// 1) Register regular routes.
	r := rpccore.NewRoutesMap(proxyService{Client: p.Client}, nil)
	rpcserver.RegisterRPCFuncs(mux, r, p.Logger)

	// 2) Allow websocket connections.
	wmLogger := p.Logger.With("protocol", "websocket")
	wm := rpcserver.NewWebsocketManager(wmLogger, r,
		rpcserver.OnDisconnect(func(remoteAddr string) {
			err := p.Client.UnsubscribeAll(context.Background(), remoteAddr)
			if err != nil && err != tmpubsub.ErrSubscriptionNotFound {
				wmLogger.Error("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
			}
		}),
		rpcserver.ReadLimit(p.Config.MaxBodyBytes),
	)

	mux.HandleFunc("/websocket", wm.WebsocketHandler)

	// 3) Start a client.
	if !p.Client.IsRunning() {
		if err := p.Client.Start(ctx); err != nil {
			return nil, mux, fmt.Errorf("can't start client: %w", err)
		}
	}

	// 4) Start listening for new connections.
	listener, err := rpcserver.Listen(p.Addr, p.Config.MaxOpenConnections)
	if err != nil {
		return nil, mux, err
	}

	return listener, mux, nil
}
