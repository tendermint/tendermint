package node

import (
	"context"
	"fmt"
	"github.com/tendermint/tendermint/internal/blocksync"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	dashquorum "github.com/tendermint/tendermint/dash/quorum"
	dashcore "github.com/tendermint/tendermint/dashcore/rpc"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/eventlog"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/proxy"
	rpccore "github.com/tendermint/tendermint/internal/rpc/core"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink"
	"github.com/tendermint/tendermint/internal/statesync"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
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

	// services
	eventSinks     []indexer.EventSink
	initialState   sm.State
	stateStore     sm.Store
	blockStore     *store.BlockStore // store the blockchain to disk
	evPool         *evidence.Pool
	indexerService *indexer.Service
	services       []service.Service
	rpcListeners   []net.Listener // rpc servers
	shutdownOps    closer
	rpcEnv         *rpccore.Environment
	prometheusSrv  *http.Server

	// Dash
	validatorConnExecutor *dashquorum.ValidatorConnExecutor
	dashCoreRPCClient     dashcore.Client
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
			logger,
			cfg,
			config.DefaultDBProvider,
			nodeKey,
			defaultGenesisDocProviderFunc(cfg),
		)
	}
	pval, err := makeDefaultPrivval(cfg)
	if err != nil {
		return nil, err
	}

	appClient, _, err := proxy.ClientFactory(logger, cfg.ProxyApp, cfg.ABCI, cfg.DBDir())
	if err != nil {
		return nil, err
	}

	return makeNode(
		ctx,
		cfg,
		pval,
		nodeKey,
		appClient,
		defaultGenesisDocProviderFunc(cfg),
		config.DefaultDBProvider,
		nil,
		logger,
	)
}

// makeNode returns a new, ready to go, Tendermint Node.
func makeNode(
	ctx context.Context,
	cfg *config.Config,
	filePrivval *privval.FilePV,
	nodeKey types.NodeKey,
	client abciclient.Client,
	genesisDocProvider genesisDocProvider,
	dbProvider config.DBProvider,
	dashCoreRPCClient dashcore.Client,
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

	if err = genDoc.ValidateAndComplete(); err != nil {
		return nil, combineCloseError(fmt.Errorf("error in genesis doc: %w", err), makeCloser(closers))
	}

	state, err := loadStateFromDBOrGenesisDocProvider(stateStore, genDoc)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	nodeMetrics := defaultMetricsProvider(cfg.Instrumentation)(genDoc.ChainID)

	proxyApp := proxy.New(client, logger.With("module", "proxy"), nodeMetrics.proxy)
	eventBus := eventbus.NewDefault(logger.With("module", "events"))

	var eventLog *eventlog.Log
	if w := cfg.RPC.EventLogWindowSize; w > 0 {
		var err error
		eventLog, err = eventlog.New(eventlog.LogSettings{
			WindowSize: w,
			MaxItems:   cfg.RPC.EventLogMaxItems,
			Metrics:    nodeMetrics.eventlog,
		})
		if err != nil {
			return nil, combineCloseError(fmt.Errorf("initializing event log: %w", err), makeCloser(closers))
		}
	}
	eventSinks, err := sink.EventSinksFromConfig(cfg, dbProvider, genDoc.ChainID)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}
	indexerService := indexer.NewService(indexer.ServiceArgs{
		Sinks:    eventSinks,
		EventBus: eventBus,
		Logger:   logger.With("module", "txindex"),
		Metrics:  nodeMetrics.indexer,
	})

	privValidator, err := createPrivval(ctx, logger, cfg, genDoc, filePrivval)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	var proTxHash crypto.ProTxHash
	switch {
	case cfg.PrivValidator.CoreRPCHost != "":
		logger.Info(
			"Initializing Dash Core Signing",
			"quorum hash",
			state.Validators.QuorumHash.String(),
		)
		/*
			llmqType := config.Consensus.QuorumType
			if llmqType == 0 {
				llmqType = btcjson.LLMQType_100_67
			}*/
		if dashCoreRPCClient == nil {
			rpcClient, err := DefaultDashCoreRPCClient(cfg, logger.With("module", dashcore.ModuleName))
			if err != nil {
				return nil, combineCloseError(
					fmt.Errorf("failed to create Dash Core RPC client %w", err),
					makeCloser(closers),
				)
			}
			dashCoreRPCClient = rpcClient
		}

		if cfg.Mode == config.ModeValidator {
			// If a local port is provided for Dash Core rpc into the service to sign.
			privValidator, err = createAndStartPrivValidatorRPCClient(
				cfg.Consensus.QuorumType,
				dashCoreRPCClient,
				logger,
			)
			if err != nil {
				return nil, combineCloseError(
					fmt.Errorf("error with private validator RPC client: %w", err),
					makeCloser(closers),
				)
			}
			proTxHash, err = privValidator.GetProTxHash(ctx)
			if err != nil {
				return nil, combineCloseError(
					fmt.Errorf("can't get proTxHash using dash core signing: %w", err),
					makeCloser(closers),
				)
			}
			logger.Info("Connected to Core RPC Masternode", "proTxHash", proTxHash.String())
		} else {
			logger.Info("Connected to Core RPC FullNode")
		}

	case cfg.PrivValidator.ListenAddr != "":
		// If an address is provided, listen on the socket for a connection from an
		// external signing process.
		// FIXME: we should start services inside OnStart
		protocol, _ := tmnet.ProtocolAndAddress(cfg.PrivValidator.ListenAddr)
		// FIXME: we should start services inside OnStart
		switch protocol {
		case "grpc":
			privValidator, err = createAndStartPrivValidatorGRPCClient(cfg, genDoc.ChainID, genDoc.QuorumHash, logger)
			if err != nil {
				return nil, combineCloseError(
					fmt.Errorf("error with private validator grpc client: %w", err),
					makeCloser(closers),
				)
			}
		default:
			privValidator, err = createAndStartPrivValidatorSocketClient(ctx, cfg.PrivValidator.ListenAddr, genDoc.ChainID, genDoc.QuorumHash, logger)
			if err != nil {
				return nil, combineCloseError(
					fmt.Errorf("error with private validator socket client: %w", err),
					makeCloser(closers),
				)
			}
		}
		if cfg.Mode == config.ModeValidator {
			proTxHash, err = privValidator.GetProTxHash(ctx)
			if err != nil {
				return nil, combineCloseError(
					fmt.Errorf("can't get proTxHash using dash core signing: %w", err),
					makeCloser(closers),
				)
			}
			logger.Info(
				"Connected to Private Validator through listen address",
				"proTxHash",
				proTxHash.String(),
			)
		} else {
			logger.Info("Connected to Private Validator through listen address")
		}
	default:
		privValidator, err = privval.LoadOrGenFilePV(
			cfg.PrivValidator.KeyFile(),
			cfg.PrivValidator.StateFile(),
		)
		if err != nil {
			return nil, combineCloseError(
				fmt.Errorf("error with private validator loaded: %w", err),
				makeCloser(closers),
			)
		}
		proTxHash, err = privValidator.GetProTxHash(ctx)
		if err != nil {
			return nil, combineCloseError(
				fmt.Errorf("can't get proTxHash through file: %w", err),
				makeCloser(closers),
			)
		}
		logger.Info("Private Validator using local file", "proTxHash", proTxHash.String())
	}

	if dashCoreRPCClient == nil {
		llmqType := cfg.Consensus.QuorumType
		if llmqType == 0 {
			llmqType = btcjson.LLMQType_100_67
		}
		// This is used for light client verification only
		dashCoreRPCClient = dashcore.NewMockClient(cfg.ChainID(), llmqType, privValidator, false)
	}

	//if cfg.Mode == config.ModeValidator {
	//	pubKey, err = privValidator.GetPubKey(ctx)
	//	if err != nil {
	//		return nil, combineCloseError(fmt.Errorf("can't get pubkey: %w", err),
	//			makeCloser(closers))
	//
	//	}
	//	if pubKey == nil {
	//		return nil, combineCloseError(
	//			errors.New("could not retrieve public key from private validator"),
	//			makeCloser(closers))
	//	}
	//	// This is used for light client verification only
	//	dashCoreRPCClient = dashcore.NewMockClient(cfg.ChainID(), llmqType, privValidator, false)
	//}

	weAreOnlyValidator := onlyValidatorIsUs(state, proTxHash)

	peerManager, peerCloser, err := createPeerManager(cfg, dbProvider, nodeKey.ID)
	closers = append(closers, peerCloser)
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("failed to create peer manager: %w", err),
			makeCloser(closers))
	}

	// Start Dash connection executor
	var validatorConnExecutor *dashquorum.ValidatorConnExecutor
	if len(proTxHash) > 0 {
		vcLogger := logger.With("node_proTxHash", proTxHash.ShortString(), "module", "ValidatorConnExecutor")
		dcm := p2p.NewRouterDashDialer(peerManager, vcLogger)
		validatorConnExecutor, err = dashquorum.NewValidatorConnExecutor(
			proTxHash,
			eventBus,
			dcm,
			dashquorum.WithLogger(vcLogger),
			dashquorum.WithValidatorsSet(state.Validators),
		)
		if err != nil {
			return nil, combineCloseError(err, makeCloser(closers))
		}
	}

	// TODO construct node here:
	node := &nodeImpl{
		config:        cfg,
		logger:        logger,
		genesisDoc:    genDoc,
		privValidator: privValidator,

		peerManager: peerManager,
		nodeKey:     nodeKey,

		eventSinks:     eventSinks,
		indexerService: indexerService,
		services:       []service.Service{eventBus},

		initialState: state,
		stateStore:   stateStore,
		blockStore:   blockStore,

		shutdownOps: makeCloser(closers),

		validatorConnExecutor: validatorConnExecutor,
		dashCoreRPCClient:     dashCoreRPCClient,

		rpcEnv: &rpccore.Environment{
			ProxyApp: proxyApp,

			StateStore: stateStore,
			BlockStore: blockStore,

			PeerManager: peerManager,

			GenDoc:     genDoc,
			EventSinks: eventSinks,
			EventBus:   eventBus,
			EventLog:   eventLog,
			Logger:     logger.With("module", "rpc"),
			Config:     *cfg.RPC,
		},
	}

	node.router, err = createRouter(logger, nodeMetrics.p2p, node.NodeInfo, nodeKey, peerManager, cfg, proxyApp)
	if err != nil {
		return nil, combineCloseError(
			fmt.Errorf("failed to create router: %w", err),
			makeCloser(closers))
	}

	evReactor, evPool, edbCloser, err := createEvidenceReactor(logger, cfg, dbProvider,
		stateStore, blockStore, peerManager.Subscribe, node.router.OpenChannel, nodeMetrics.evidence, eventBus)
	closers = append(closers, edbCloser)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}
	node.services = append(node.services, evReactor)
	node.rpcEnv.EvidencePool = evPool
	node.evPool = evPool

	mpReactor, mp := createMempoolReactor(logger, cfg, proxyApp, stateStore, nodeMetrics.mempool,
		peerManager.Subscribe, node.router.OpenChannel, peerManager.GetHeight)
	node.rpcEnv.Mempool = mp
	node.services = append(node.services, mpReactor)

	nextCoreChainLock, err := types.CoreChainLockFromProto(genDoc.InitialProposalCoreChainLock)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}

	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger.With("module", "state"),
		proxyApp,
		mp,
		evPool,
		blockStore,
		eventBus,
		nodeMetrics.state,
		nextCoreChainLock,
	)

	// Determine whether we should attempt state sync.
	stateSync := cfg.StateSync.Enable && !weAreOnlyValidator
	if stateSync && state.LastBlockHeight > 0 {
		logger.Info("Found local state with non-zero height, skipping state sync")
		stateSync = false
	}

	// Determine whether we should do block sync. This must happen after the handshake, since the
	// app may modify the validator set, specifying ourself as the only validator.
	blockSync := !onlyValidatorIsUs(state, proTxHash) // TODO compare with this: blockSync := cfg.BlockSync.Enable && !weAreOnlyValidator
	waitSync := stateSync || blockSync

	csState, err := consensus.NewState(logger.With("module", "consensus"),
		cfg.Consensus,
		stateStore,
		blockExec,
		blockStore,
		mp,
		evPool,
		eventBus,
		0, // TODO put a correct version
		consensus.StateMetrics(nodeMetrics.consensus),
		consensus.SkipStateStoreBootstrap,
	)
	if err != nil {
		return nil, combineCloseError(err, makeCloser(closers))
	}
	node.rpcEnv.ConsensusState = csState

	csReactor := consensus.NewReactor(
		logger,
		csState,
		node.router.OpenChannel,
		peerManager.Subscribe,
		eventBus,
		waitSync,
		nodeMetrics.consensus,
	)
	node.services = append(node.services, csReactor)
	node.rpcEnv.ConsensusReactor = csReactor

	// Create the blockchain reactor. Note, we do not start block sync if we're
	// doing a state sync first.
	bcReactor := blocksync.NewReactor(
		logger.With("module", "blockchain"),
		stateStore,
		blockExec,
		blockStore,
		proTxHash,
		csReactor,
		node.router.OpenChannel,
		peerManager.Subscribe,
		blockSync && !stateSync,
		nodeMetrics.consensus,
		eventBus,
	)
	node.services = append(node.services, bcReactor)
	node.rpcEnv.BlockSyncReactor = bcReactor

	// Make ConsensusReactor. Don't enable fully if doing a state sync and/or block sync first.
	// FIXME We need to update metrics here, since other reactors don't have access to them.
	if stateSync {
		nodeMetrics.consensus.StateSyncing.Set(1)
	} else if blockSync {
		nodeMetrics.consensus.BlockSyncing.Set(1)
	}

	if cfg.P2P.PexReactor {
		node.services = append(node.services, pex.NewReactor(logger, peerManager, node.router.OpenChannel, peerManager.Subscribe))
	}

	// Set up state sync reactor, and schedule a sync if requested.
	// FIXME The way we do phased startups (e.g. replay -> block sync -> consensus) is very messy,
	// we should clean this whole thing up. See:
	// https://github.com/tendermint/tendermint/issues/4644
	node.services = append(node.services, statesync.NewReactor(
		genDoc.ChainID,
		genDoc.InitialHeight,
		*cfg.StateSync,
		logger.With("module", "statesync"),
		proxyApp,
		node.router.OpenChannel,
		peerManager.Subscribe,
		stateStore,
		blockStore,
		cfg.StateSync.TempDir,
		nodeMetrics.statesync,
		eventBus,
		// the post-sync operation
		func(ctx context.Context, state sm.State) error {
			csReactor.SetStateSyncingMetrics(0)

			// TODO: Some form of orchestrator is needed here between the state
			// advancing reactors to be able to control which one of the three
			// is running
			// FIXME Very ugly to have these metrics bleed through here.
			csReactor.SetBlockSyncingMetrics(1)
			if err := bcReactor.SwitchToBlockSync(ctx, state); err != nil {
				logger.Error("failed to switch to block sync", "err", err)
				return err
			}

			return nil
		},
		false, // TODO pass a correct value
		dashCoreRPCClient,
		stateSync,
	))

	if cfg.Mode == config.ModeValidator {
		if privValidator != nil {
			csState.SetPrivValidator(ctx, privValidator)
		}
		node.rpcEnv.PubKey = pubKey
	}

	node.BaseService = *service.NewBaseService(logger, "Node", node)

	return node, nil
}

// OnStart starts the Node. It implements service.Service.
func (n *nodeImpl) OnStart(ctx context.Context) error {
	if err := n.rpcEnv.ProxyApp.Start(ctx); err != nil {
		return fmt.Errorf("error starting proxy app connections: %w", err)
	}

	proTxHash, err := n.privValidator.GetProTxHash(ctx)
	if err != nil {
		return err
	}

	// EventBus and IndexerService must be started before the handshake because
	// we might need to index the txs of the replayed block as this might not have happened
	// when the node stopped last time (i.e. the node stopped or crashed after it saved the block
	// but before it indexed the txs)
	if err := n.rpcEnv.EventBus.Start(ctx); err != nil {
		return err
	}

	if err := n.indexerService.Start(ctx); err != nil {
		return err
	}

	// Create the handshaker, which calls RequestInfo, sets the AppVersion on the state,
	// and replays any blocks as necessary to sync tendermint with the app.
	handshaker := consensus.NewHandshaker(n.logger.With("module", "handshaker"),
		n.stateStore, n.initialState, n.blockStore, n.rpcEnv.EventBus, n.genesisDoc,
		proTxHash, n.config.Consensus.AppHashSize,
	)
	// TODO: populate a consensus state with "proposedVersion"
	proposedVersion, err := handshaker.Handshake(ctx, n.rpcEnv.ProxyApp)
	if err != nil {
		return err
	}

	// Reload the state. It will have the Version.Consensus.App set by the
	// Handshake, and may have other modifications as well (ie. depending on
	// what happened during block replay).
	state, err := n.stateStore.Load()
	if err != nil {
		return fmt.Errorf("cannot load state: %w", err)
	}

	logNodeStartupInfo(state, proTxHash, n.logger, n.logger, n.config.Mode)

	// TODO: Fetch and provide real options and do proper p2p bootstrapping.
	// TODO: Use a persistent peer database.
	n.nodeInfo, err = makeNodeInfo(n.config, n.nodeKey, n.eventSinks, n.genesisDoc, state.Version.Consensus)
	if err != nil {
		return err
	}
	// Start Internal Services

	if n.config.RPC.PprofListenAddress != "" {
		signal := make(chan struct{})
		srv := &http.Server{Addr: n.config.RPC.PprofListenAddress, Handler: nil}
		go func() {
			select {
			case <-ctx.Done():
				sctx, scancel := context.WithTimeout(context.Background(), time.Second)
				defer scancel()
				_ = srv.Shutdown(sctx)
			case <-signal:
			}
		}()

		go func() {
			n.logger.Info("Starting pprof server", "laddr", n.config.RPC.PprofListenAddress)

			if err := srv.ListenAndServe(); err != nil {
				n.logger.Error("pprof server error", "err", err)
				close(signal)
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

	state, err = n.stateStore.Load()
	if err != nil {
		return err
	}
	if err := n.evPool.Start(state); err != nil {
		return err
	}

	if n.config.Instrumentation.Prometheus && n.config.Instrumentation.PrometheusListenAddr != "" {
		n.prometheusSrv = n.startPrometheusServer(ctx, n.config.Instrumentation.PrometheusListenAddr)
	}

	// Start the transport.
	if err := n.router.Start(ctx); err != nil {
		return err
	}
	n.rpcEnv.IsListening = true

	for _, reactor := range n.services {
		if err := reactor.Start(ctx); err != nil {
			return fmt.Errorf("problem starting service '%T': %w ", reactor, err)
		}
	}

	n.rpcEnv.NodeInfo = n.nodeInfo
	// Start the RPC server before the P2P server
	// so we can eg. receive txs for the first block
	if n.config.RPC.ListenAddress != "" {
		var err error
		n.rpcListeners, err = n.rpcEnv.StartService(ctx, n.config)
		if err != nil {
			return err
		}
	}

	return nil
}

// OnStop stops the Node. It implements service.Service.
func (n *nodeImpl) OnStop() {
	n.logger.Info("Stopping Node")
	// stop the listeners / external services first
	for _, l := range n.rpcListeners {
		n.logger.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			n.logger.Error("error closing listener", "listener", l, "err", err)
		}
	}

	for _, es := range n.eventSinks {
		if err := es.Stop(); err != nil {
			n.logger.Error("failed to stop event sink", "err", err)
		}
	}

	for _, reactor := range n.services {
		reactor.Wait()
	}

	n.router.Wait()
	n.rpcEnv.IsListening = false

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

	signal := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sctx, scancel := context.WithTimeout(context.Background(), time.Second)
			defer scancel()
			_ = srv.Shutdown(sctx)
		case <-signal:
		}
	}()

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			n.logger.Error("Prometheus HTTP server ListenAndServe", "err", err)
			close(signal)
		}
	}()

	return srv
}

func (n *nodeImpl) NodeInfo() *types.NodeInfo {
	return &n.nodeInfo
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
func loadStateFromDBOrGenesisDocProvider(stateStore sm.Store, genDoc *types.GenesisDoc) (sm.State, error) {

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

		// 3. save the gensis document to the state store so
		// its fetchable by other callers.
		if err := stateStore.Save(state); err != nil {
			return sm.State{}, err
		}
	}

	return state, nil
}

func getRouterConfig(conf *config.Config, appClient abciclient.Client) p2p.RouterOptions {
	opts := p2p.RouterOptions{
		QueueType: conf.P2P.QueueType,
	}

	if conf.FilterPeers && appClient != nil {
		opts.FilterPeerByID = func(ctx context.Context, id types.NodeID) error {
			res, err := appClient.Query(ctx, &abci.RequestQuery{
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
			res, err := appClient.Query(ctx, &abci.RequestQuery{
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

// DefaultDashCoreRPCClient returns RPC client for the Dash Core node
func DefaultDashCoreRPCClient(cfg *config.Config, logger log.Logger) (dashcore.Client, error) {
	return dashcore.NewRPCClient(
		cfg.PrivValidator.CoreRPCHost,
		cfg.PrivValidator.CoreRPCUsername,
		cfg.PrivValidator.CoreRPCPassword,
		logger,
	)
}

func createAndStartPrivValidatorSocketClient(
	ctx context.Context,
	listenAddr,
	chainID string,
	quorumHash crypto.QuorumHash,
	logger log.Logger,
) (types.PrivValidator, error) {

	pve, err := privval.NewSignerListener(listenAddr, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start private validator: %w", err)
	}

	pvsc, err := privval.NewSignerClient(ctx, pve, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to start private validator: %w", err)
	}

	// try to get a pubkey from private validate first time
	_, err = pvsc.GetPubKey(context.TODO(), quorumHash)
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}

	const (
		retries = 50 // 50 * 100ms = 5s total
		timeout = 100 * time.Millisecond
	)
	pvscWithRetries := privval.NewRetrySignerClient(pvsc, retries, timeout)

	return pvscWithRetries, nil
}

func createAndStartPrivValidatorRPCClient(
	defaultQuorumType btcjson.LLMQType,
	dashCoreRPCClient dashcore.Client,
	logger log.Logger,
) (types.PrivValidator, error) {
	pvsc, err := privval.NewDashCoreSignerClient(dashCoreRPCClient, defaultQuorumType)
	if err != nil {
		return nil, fmt.Errorf("failed to start private validator: %w", err)
	}

	// try to ping Core from private validator first time to make sure connection works
	err = pvsc.Ping()
	if err != nil {
		return nil, fmt.Errorf(
			"can't ping core server when starting private validator rpc client: %w",
			err,
		)
	}

	return pvsc, nil
}

func createAndStartPrivValidatorGRPCClient(
	cfg *config.Config,
	chainID string,
	quorumHash crypto.QuorumHash,
	logger log.Logger,
) (types.PrivValidator, error) {
	pvsc, err := tmgrpc.DialRemoteSigner(
		cfg.PrivValidator,
		chainID,
		logger,
		cfg.Instrumentation.Prometheus,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start private validator: %w", err)
	}

	// try to get a pubkey from private validate first time
	_, err = pvsc.GetPubKey(context.TODO(), quorumHash)
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}

	return pvsc, nil
}
