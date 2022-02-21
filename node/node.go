package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/blocksync"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/eventlog"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	"github.com/tendermint/tendermint/internal/proxy"
	rpccore "github.com/tendermint/tendermint/internal/rpc/core"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/statesync"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"

	_ "net/http/pprof" // nolint: gosec // securely exposed on separate, optional port

	_ "github.com/lib/pq" // provide the psql db driver
)

// nodeImpl is the highest level interface to a full Tendermint node.
// It includes all configuration information and running services.
type nodeImpl struct {
	service.BaseService
	logger log.Logger

	// config
	config        *config.Config
	genesisDoc    *types.GenesisDoc   // initial validator set
	privValidator types.PrivValidator // local node's validator key

	// network
	peerManager *p2p.PeerManager
	router      *p2p.Router
	nodeInfo    types.NodeInfo
	nodeKey     types.NodeKey // our node privkey
	isListening bool

	// services
	eventSinks       []indexer.EventSink
	stateStore       sm.Store
	blockStore       *store.BlockStore  // store the blockchain to disk
	stateSync        bool               // whether the node should state sync on startup
	stateSyncReactor *statesync.Reactor // for hosting and restoring state sync snapshots

	services      []service.Service
	rpcListeners  []net.Listener // rpc servers
	shutdownOps   closer
	rpcEnv        *rpccore.Environment
	prometheusSrv *http.Server
}

// newDefaultNode returns a Tendermint node with default settings for the
// PrivValidator, ClientCreator, GenesisDoc, and DBProvider.
// It implements NodeProvider.
func newDefaultNode(
	ctx context.Context,
	cfg *config.Config,
	logger log.Logger,
) (service.Service, error) {
	nodeKey, err := types.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load or gen node key %s: %w", cfg.NodeKeyFile(), err)
	}
	if cfg.Mode == config.ModeSeed {
		return makeSeedNode(
			ctx,
			cfg,
			config.DefaultDBProvider,
			nodeKey,
			defaultGenesisDocProviderFunc(cfg),
			logger,
		)
	}
	pval, err := makeDefaultPrivval(cfg)
	if err != nil {
		return nil, err
	}

	appClient, _ := proxy.DefaultClientCreator(logger, cfg.ProxyApp, cfg.ABCI, cfg.DBDir())

	return makeNode(
		ctx,
		cfg,
		pval,
		nodeKey,
		appClient,
		defaultGenesisDocProviderFunc(cfg),
		config.DefaultDBProvider,
		logger,
	)
}

// makeNode returns a new, ready to go, Tendermint Node.
func makeNode(
	ctx context.Context,
	cfg *config.Config,
	filePrivval *privval.FilePV,
	nodeKey types.NodeKey,
	clientCreator abciclient.Creator,
	genesisDocProvider genesisDocProvider,
	dbProvider config.DBProvider,
	logger log.Logger,
) (service.Service, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	closers := []closer{convertCancelCloser(cancel)}

	blockStore, stateDB, dbCloser, err := initDBs(cfg, dbProvider)
	if err != nil {
		return nil, combineCloseError(err, dbCloser)
	}
	closers = append(closers, dbCloser)

	stateStore := sm.NewStore(stateDB)

	genDoc, err := genesisDocProvider()
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	err = genDoc.ValidateAndComplete()
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("error in genesis doc: %w", err),
			makeCloser(closers))
	}

	state, err := loadStateFromDBOrGenesisDocProvider(stateStore, genDoc)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	nodeMetrics := defaultMetricsProvider(cfg.Instrumentation)(genDoc.ChainID)

	// Create the proxyApp and establish connections to the ABCI app (consensus, mempool, query).
	proxyApp := proxy.NewAppConns(clientCreator, logger.With("module", "proxy"), nodeMetrics.proxy)
	if err := proxyApp.Start(ctx); err != nil {
		return nil, fmt.Errorf("error starting proxy app connections: %w", err)
	}

	// EventBus and IndexerService must be started before the handshake because
	// we might need to index the txs of the replayed block as this might not have happened
	// when the node stopped last time (i.e. the node stopped after it saved the block
	// but before it indexed the txs, or, endblocker panicked)
	eventBus := eventbus.NewDefault(logger.With("module", "events"))
	if err := eventBus.Start(ctx); err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	var eventLog *eventlog.Log
	if w := cfg.RPC.EventLogWindowSize; w > 0 {
		var err error
		eventLog, err = eventlog.New(eventlog.LogSettings{
			WindowSize: w,
			MaxItems:   cfg.RPC.EventLogMaxItems,
			Metrics:    nodeMetrics.eventlog,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing event log: %w", err)
		}
	}

	indexerService, eventSinks, err := createAndStartIndexerService(
		ctx, cfg, dbProvider, eventBus,
		logger, genDoc.ChainID, nodeMetrics.indexer)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}
	closers = append(closers, func() error { indexerService.Stop(); return nil })

	privValidator, err := createPrivval(ctx, logger, cfg, genDoc, filePrivval)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	var pubKey crypto.PubKey
	if cfg.Mode == config.ModeValidator {
		pubKey, err = privValidator.GetPubKey(ctx)
		if err != nil {
			return nil, combineCloseError(fmt.Errorf("can't get pubkey: %w", err),
				makeCloser(closers))

		}
		if pubKey == nil {
			return nil, combineCloseError(
				errors.New("could not retrieve public key from private validator"),
				makeCloser(closers))
		}
	}

	// Determine whether we should attempt state sync.
	stateSync := cfg.StateSync.Enable && !onlyValidatorIsUs(state, pubKey)
	if stateSync && state.LastBlockHeight > 0 {
		logger.Info("Found local state with non-zero height, skipping state sync")
		stateSync = false
	}

	// Create the handshaker, which calls RequestInfo, sets the AppVersion on the state,
	// and replays any blocks as necessary to sync tendermint with the app.
	if !stateSync {
		if err := consensus.NewHandshaker(
			logger.With("module", "handshaker"),
			stateStore, state, blockStore, eventBus, genDoc,
		).Handshake(ctx, proxyApp); err != nil {
			return nil, combineCloseError(err, makeCloser(closers))
		}

		// Reload the state. It will have the Version.Consensus.App set by the
		// Handshake, and may have other modifications as well (ie. depending on
		// what happened during block replay).
		state, err = stateStore.Load()
		if err != nil {
			return nil, combineCloseError(
				fmt.Errorf("cannot load state: %w", err),
				makeCloser(closers))
		}
	}

	// Determine whether we should do block sync. This must happen after the handshake, since the
	// app may modify the validator set, specifying ourself as the only validator.
	blockSync := !onlyValidatorIsUs(state, pubKey)

	logNodeStartupInfo(state, pubKey, logger, cfg.Mode)

	// TODO: Fetch and provide real options and do proper p2p bootstrapping.
	// TODO: Use a persistent peer database.
	nodeInfo, err := makeNodeInfo(cfg, nodeKey, eventSinks, genDoc, state)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	peerManager, peerCloser, err := createPeerManager(cfg, dbProvider, nodeKey.ID)
	closers = append(closers, peerCloser)
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("failed to create peer manager: %w", err),
			makeCloser(closers))
	}

	router, err := createRouter(ctx, logger, nodeMetrics.p2p, nodeInfo, nodeKey,
		peerManager, cfg, proxyApp)
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("failed to create router: %w", err),
			makeCloser(closers))
	}

	mpReactor, mp, err := createMempoolReactor(ctx,
		cfg, proxyApp, state, nodeMetrics.mempool, peerManager, router, logger,
	)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	evReactor, evPool, err := createEvidenceReactor(ctx,
		cfg, dbProvider, stateDB, blockStore, peerManager, router, logger, nodeMetrics.evidence, eventBus,
	)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger.With("module", "state"),
		proxyApp.Consensus(),
		mp,
		evPool,
		blockStore,
		sm.BlockExecutorWithMetrics(nodeMetrics.state),
	)

	csReactor, csState, err := createConsensusReactor(ctx,
		cfg, state, blockExec, blockStore, mp, evPool,
		privValidator, nodeMetrics.consensus, stateSync || blockSync, eventBus,
		peerManager, router, logger,
	)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	// Create the blockchain reactor. Note, we do not start block sync if we're
	// doing a state sync first.
	bcReactor, err := blocksync.NewReactor(ctx,
		logger.With("module", "blockchain"),
		state.Copy(),
		blockExec,
		blockStore,
		csReactor,
		router.OpenChannel,
		peerManager.Subscribe(ctx),
		blockSync && !stateSync,
		nodeMetrics.consensus,
		eventBus,
	)
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("could not create blocksync reactor: %w", err),
			makeCloser(closers))
	}

	// Make ConsensusReactor. Don't enable fully if doing a state sync and/or block sync first.
	// FIXME We need to update metrics here, since other reactors don't have access to them.
	if stateSync {
		nodeMetrics.consensus.StateSyncing.Set(1)
	} else if blockSync {
		nodeMetrics.consensus.BlockSyncing.Set(1)
	}

	// Set up state sync reactor, and schedule a sync if requested.
	// FIXME The way we do phased startups (e.g. replay -> block sync -> consensus) is very messy,
	// we should clean this whole thing up. See:
	// https://github.com/tendermint/tendermint/issues/4644
	stateSyncReactor, err := statesync.NewReactor(
		ctx,
		genDoc.ChainID,
		genDoc.InitialHeight,
		*cfg.StateSync,
		logger.With("module", "statesync"),
		proxyApp.Snapshot(),
		proxyApp.Query(),
		router.OpenChannel,
		peerManager.Subscribe(ctx),
		stateStore,
		blockStore,
		cfg.StateSync.TempDir,
		nodeMetrics.statesync,
		eventBus,
	)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	var pexReactor service.Service
	if cfg.P2P.PexReactor {
		pexReactor, err = pex.NewReactor(ctx, logger, peerManager, router.OpenChannel, peerManager.Subscribe(ctx))
		if err != nil {
			return nil, combineCloseError(err, makeCloser(closers))
		}
	}
	node := &nodeImpl{
		config:        cfg,
		logger:        logger,
		genesisDoc:    genDoc,
		privValidator: privValidator,

		peerManager: peerManager,
		router:      router,
		nodeInfo:    nodeInfo,
		nodeKey:     nodeKey,

		eventSinks: eventSinks,

		services: []service.Service{
			eventBus,
			evReactor,
			mpReactor,
			csReactor,
			bcReactor,
			pexReactor,
		},

		stateStore:       stateStore,
		blockStore:       blockStore,
		stateSyncReactor: stateSyncReactor,
		stateSync:        stateSync,

		shutdownOps: makeCloser(closers),

		rpcEnv: &rpccore.Environment{
			ProxyAppQuery:   proxyApp.Query(),
			ProxyAppMempool: proxyApp.Mempool(),

			StateStore:     stateStore,
			BlockStore:     blockStore,
			EvidencePool:   evPool,
			ConsensusState: csState,

			ConsensusReactor: csReactor,
			BlockSyncReactor: bcReactor,

			PeerManager: peerManager,

			GenDoc:     genDoc,
			EventSinks: eventSinks,
			EventBus:   eventBus,
			EventLog:   eventLog,
			Mempool:    mp,
			Logger:     logger.With("module", "rpc"),
			Config:     *cfg.RPC,
		},
	}

	if cfg.Mode == config.ModeValidator {
		node.rpcEnv.PubKey = pubKey
	}

	node.rpcEnv.P2PTransport = node

	node.BaseService = *service.NewBaseService(logger, "Node", node)

	return node, nil
}

// OnStart starts the Node. It implements service.Service.
func (n *nodeImpl) OnStart(ctx context.Context) error {
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

		timer := time.NewTimer(genTime.Sub(now))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	// Start the RPC server before the P2P server
	// so we can eg. receive txs for the first block
	if n.config.RPC.ListenAddress != "" {
		var err error
		n.rpcListeners, err = n.rpcEnv.StartService(ctx, n.config)
		if err != nil {
			return err
		}
	}

	if n.config.Instrumentation.Prometheus && n.config.Instrumentation.PrometheusListenAddr != "" {
		n.prometheusSrv = n.startPrometheusServer(ctx, n.config.Instrumentation.PrometheusListenAddr)
	}

	// Start the transport.
	if err := n.router.Start(ctx); err != nil {
		return err
	}
	n.isListening = true

	for _, reactor := range n.services {
		if err := reactor.Start(ctx); err != nil {
			return fmt.Errorf("problem starting service '%T': %w ", reactor, err)
		}
	}

	if err := n.stateSyncReactor.Start(ctx); err != nil {
		return err
	}

	// Run state sync
	// TODO: We shouldn't run state sync if we already have state that has a
	// LastBlockHeight that is not InitialHeight
	if n.stateSync {
		bcR := n.rpcEnv.BlockSyncReactor

		// we need to get the genesis state to get parameters such as
		state, err := sm.MakeGenesisState(n.genesisDoc)
		if err != nil {
			return fmt.Errorf("unable to derive state: %w", err)
		}

		// TODO: we may want to move these events within the respective
		// reactors.
		// At the beginning of the statesync start, we use the initialHeight as the event height
		// because of the statesync doesn't have the concreate state height before fetched the snapshot.
		d := types.EventDataStateSyncStatus{Complete: false, Height: state.InitialHeight}
		if err := n.stateSyncReactor.PublishStatus(ctx, d); err != nil {
			n.logger.Error("failed to emit the statesync start event", "err", err)
		}

		// RUN STATE SYNC NOW:
		//
		// TODO: Eventually this should run as part of some
		// separate orchestrator
		n.logger.Info("starting state sync")
		ssState, err := n.stateSyncReactor.Sync(ctx)
		if err != nil {
			n.logger.Error("state sync failed; shutting down this node", "err", err)
			// stop the node
			n.Stop()
			return err
		}

		n.rpcEnv.ConsensusReactor.SetStateSyncingMetrics(0)

		if err := n.stateSyncReactor.PublishStatus(ctx,
			types.EventDataStateSyncStatus{
				Complete: true,
				Height:   ssState.LastBlockHeight,
			}); err != nil {
			n.logger.Error("failed to emit the statesync start event", "err", err)
			return err
		}

		// TODO: Some form of orchestrator is needed here between the state
		// advancing reactors to be able to control which one of the three
		// is running
		// FIXME Very ugly to have these metrics bleed through here.
		n.rpcEnv.ConsensusReactor.SetBlockSyncingMetrics(1)
		if err := bcR.SwitchToBlockSync(ctx, ssState); err != nil {
			n.logger.Error("failed to switch to block sync", "err", err)
			return err
		}

		if err := bcR.PublishStatus(ctx,
			types.EventDataBlockSyncStatus{
				Complete: false,
				Height:   ssState.LastBlockHeight,
			}); err != nil {
			n.logger.Error("failed to emit the block sync starting event", "err", err)
			return err
		}
	}

	return nil
}

// OnStop stops the Node. It implements service.Service.
func (n *nodeImpl) OnStop() {
	n.logger.Info("Stopping Node")
	for _, es := range n.eventSinks {
		if err := es.Stop(); err != nil {
			n.logger.Error("failed to stop event sink", "err", err)
		}
	}

	for _, reactor := range n.services {
		reactor.Wait()
	}

	n.stateSyncReactor.Wait()
	n.router.Wait()
	n.isListening = false

	// finally stop the listeners / external services
	for _, l := range n.rpcListeners {
		n.logger.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			n.logger.Error("error closing listener", "listener", l, "err", err)
		}
	}

	if pvsc, ok := n.privValidator.(service.Service); ok {
		pvsc.Wait()
	}

	if n.prometheusSrv != nil {
		if err := n.prometheusSrv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			n.logger.Error("Prometheus HTTP server Shutdown", "err", err)
		}

	}
	if err := n.shutdownOps(); err != nil {
		if strings.TrimSpace(err.Error()) != "" {
			n.logger.Error("problem shutting down additional services", "err", err)
		}
	}
	if n.blockStore != nil {
		if err := n.blockStore.Close(); err != nil {
			n.logger.Error("problem closing blockstore", "err", err)
		}
	}
	if n.stateStore != nil {
		if err := n.stateStore.Close(); err != nil {
			n.logger.Error("problem closing statestore", "err", err)
		}
	}
}

// startPrometheusServer starts a Prometheus HTTP server, listening for metrics
// collectors on addr.
func (n *nodeImpl) startPrometheusServer(ctx context.Context, addr string) *http.Server {
	srv := &http.Server{
		Addr: addr,
		Handler: promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{MaxRequestsInFlight: n.config.Instrumentation.MaxOpenConnections},
			),
		),
	}

	promCtx, promCancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-ctx.Done():
			sctx, scancel := context.WithTimeout(context.Background(), time.Second)
			defer scancel()
			_ = srv.Shutdown(sctx)
		case <-promCtx.Done():
		}
	}()

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			n.logger.Error("Prometheus HTTP server ListenAndServe", "err", err)
			promCancel()
		}
	}()

	return srv
}

// EventBus returns the Node's EventBus.
func (n *nodeImpl) EventBus() *eventbus.EventBus {
	return n.rpcEnv.EventBus
}

// GenesisDoc returns the Node's GenesisDoc.
func (n *nodeImpl) GenesisDoc() *types.GenesisDoc {
	return n.genesisDoc
}

// RPCEnvironment makes sure RPC has all the objects it needs to operate.
func (n *nodeImpl) RPCEnvironment() *rpccore.Environment {
	return n.rpcEnv
}

//------------------------------------------------------------------------------

func (n *nodeImpl) Listeners() []string {
	return []string{
		fmt.Sprintf("Listener(@%v)", n.config.P2P.ExternalAddress),
	}
}

func (n *nodeImpl) IsListening() bool {
	return n.isListening
}

// NodeInfo returns the Node's Info from the Switch.
func (n *nodeImpl) NodeInfo() types.NodeInfo {
	return n.nodeInfo
}

// genesisDocProvider returns a GenesisDoc.
// It allows the GenesisDoc to be pulled from sources other than the
// filesystem, for instance from a distributed key-value store cluster.
type genesisDocProvider func() (*types.GenesisDoc, error)

// defaultGenesisDocProviderFunc returns a GenesisDocProvider that loads
// the GenesisDoc from the config.GenesisFile() on the filesystem.
func defaultGenesisDocProviderFunc(cfg *config.Config) genesisDocProvider {
	return func() (*types.GenesisDoc, error) {
		return types.GenesisDocFromFile(cfg.GenesisFile())
	}
}

type nodeMetrics struct {
	consensus *consensus.Metrics
	eventlog  *eventlog.Metrics
	indexer   *indexer.Metrics
	mempool   *mempool.Metrics
	p2p       *p2p.Metrics
	proxy     *proxy.Metrics
	state     *sm.Metrics
	statesync *statesync.Metrics
	evidence  *evidence.Metrics
}

// metricsProvider returns consensus, p2p, mempool, state, statesync Metrics.
type metricsProvider func(chainID string) *nodeMetrics

// defaultMetricsProvider returns Metrics build using Prometheus client library
// if Prometheus is enabled. Otherwise, it returns no-op Metrics.
func defaultMetricsProvider(cfg *config.InstrumentationConfig) metricsProvider {
	return func(chainID string) *nodeMetrics {
		if cfg.Prometheus {
			return &nodeMetrics{
				consensus: consensus.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				eventlog:  eventlog.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				indexer:   indexer.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				mempool:   mempool.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				p2p:       p2p.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				proxy:     proxy.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				state:     sm.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				statesync: statesync.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				evidence:  evidence.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
			}
		}
		return &nodeMetrics{
			consensus: consensus.NopMetrics(),
			indexer:   indexer.NopMetrics(),
			mempool:   mempool.NopMetrics(),
			p2p:       p2p.NopMetrics(),
			proxy:     proxy.NopMetrics(),
			state:     sm.NopMetrics(),
			statesync: statesync.NopMetrics(),
			evidence:  evidence.NopMetrics(),
		}
	}
}

//------------------------------------------------------------------------------

// loadStateFromDBOrGenesisDocProvider attempts to load the state from the
// database, or creates one using the given genesisDocProvider. On success this also
// returns the genesis doc loaded through the given provider.
func loadStateFromDBOrGenesisDocProvider(
	stateStore sm.Store,
	genDoc *types.GenesisDoc,
) (sm.State, error) {

	// 1. Attempt to load state form the database
	state, err := stateStore.Load()
	if err != nil {
		return sm.State{}, err
	}

	if state.IsEmpty() {
		// 2. If it's not there, derive it from the genesis doc
		state, err = sm.MakeGenesisState(genDoc)
		if err != nil {
			return sm.State{}, err
		}
	}

	return state, nil
}

func getRouterConfig(conf *config.Config, proxyApp proxy.AppConns) p2p.RouterOptions {
	opts := p2p.RouterOptions{
		QueueType: conf.P2P.QueueType,
	}

	if conf.FilterPeers && proxyApp != nil {
		opts.FilterPeerByID = func(ctx context.Context, id types.NodeID) error {
			res, err := proxyApp.Query().Query(ctx, abci.RequestQuery{
				Path: fmt.Sprintf("/p2p/filter/id/%s", id),
			})
			if err != nil {
				return err
			}
			if res.IsErr() {
				return fmt.Errorf("error querying abci app: %v", res)
			}

			return nil
		}

		opts.FilterPeerByIP = func(ctx context.Context, ip net.IP, port uint16) error {
			res, err := proxyApp.Query().Query(ctx, abci.RequestQuery{
				Path: fmt.Sprintf("/p2p/filter/addr/%s", net.JoinHostPort(ip.String(), strconv.Itoa(int(port)))),
			})
			if err != nil {
				return err
			}
			if res.IsErr() {
				return fmt.Errorf("error querying abci app: %v", res)
			}

			return nil
		}

	}

	return opts
}
