package node

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/types"
)

type seedNodeImpl struct {
	service.BaseService
	logger log.Logger

	// config
	config     *config.Config
	genesisDoc *types.GenesisDoc // initial validator set

	// network
	peerManager *p2p.PeerManager
	router      *p2p.Router
	nodeKey     types.NodeKey // our node privkey
	isListening bool

	// services
	pexReactor  service.Service // for exchanging peer addresses
	shutdownOps closer
}

// makeSeedNode returns a new seed node, containing only p2p, pex reactor
func makeSeedNode(
	logger log.Logger,
	cfg *config.Config,
	dbProvider config.DBProvider,
	nodeKey types.NodeKey,
	genesisDocProvider genesisDocProvider,
) (service.Service, error) {
	if !cfg.P2P.PexReactor {
		return nil, errors.New("cannot run seed nodes with PEX disabled")
	}

	genDoc, err := genesisDocProvider()
	if err != nil {
		return nil, err
	}

	state, err := sm.MakeGenesisState(genDoc)
	if err != nil {
		return nil, err
	}

	nodeInfo, err := makeSeedNodeInfo(cfg, nodeKey, genDoc, state)
	if err != nil {
		return nil, err
	}

	// Setup Transport and Switch.
	p2pMetrics := p2p.PrometheusMetrics(cfg.Instrumentation.Namespace, "chain_id", genDoc.ChainID)

	peerManager, closer, err := createPeerManager(cfg, dbProvider, nodeKey.ID, p2pMetrics)
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("failed to create peer manager: %w", err),
			closer)
	}

	router, err := createRouter(logger, p2pMetrics, func() *types.NodeInfo { return &nodeInfo }, nodeKey, peerManager, cfg, nil)
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("failed to create router: %w", err),
			closer)
	}

	node := &seedNodeImpl{
		config:     cfg,
		logger:     logger,
		genesisDoc: genDoc,

		nodeKey:     nodeKey,
		peerManager: peerManager,
		router:      router,

		shutdownOps: closer,

		pexReactor: pex.NewReactor(logger, peerManager, router.OpenChannel, peerManager.Subscribe),
	}
	node.BaseService = *service.NewBaseService(logger, "SeedNode", node)

	return node, nil
}

// OnStart starts the Seed Node. It implements service.Service.
func (n *seedNodeImpl) OnStart(ctx context.Context) error {
	if n.config.RPC.PprofListenAddress != "" {
		rpcCtx, rpcCancel := context.WithCancel(ctx)
		srv := &http.Server{Addr: n.config.RPC.PprofListenAddress, Handler: nil}
		go func() {
			select {
			case <-ctx.Done():
				sctx, scancel := context.WithTimeout(context.Background(), time.Second)
				defer scancel()
				_ = srv.Shutdown(sctx)
			case <-rpcCtx.Done():
			}
		}()

		go func() {
			n.logger.Info("Starting pprof server", "laddr", n.config.RPC.PprofListenAddress)

			if err := srv.ListenAndServe(); err != nil {
				n.logger.Error("pprof server error", "err", err)
				rpcCancel()
			}
		}()
	}

	now := tmtime.Now()
	genTime := n.genesisDoc.GenesisTime
	if genTime.After(now) {
		n.logger.Info("Genesis time is in the future. Sleeping until then...", "genTime", genTime)
		time.Sleep(genTime.Sub(now))
	}

	// Start the transport.
	if err := n.router.Start(ctx); err != nil {
		return err
	}
	n.isListening = true

	if n.config.P2P.PexReactor {
		if err := n.pexReactor.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

// OnStop stops the Seed Node. It implements service.Service.
func (n *seedNodeImpl) OnStop() {
	n.logger.Info("Stopping Node")

	n.pexReactor.Wait()
	n.router.Wait()
	n.isListening = false

	if err := n.shutdownOps(); err != nil {
		if strings.TrimSpace(err.Error()) != "" {
			n.logger.Error("problem shutting down additional services", "err", err)
		}
	}
}
