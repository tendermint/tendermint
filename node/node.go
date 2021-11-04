package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	"github.com/tendermint/tendermint/internal/proxy"
	rpccore "github.com/tendermint/tendermint/internal/rpc/core"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/statesync"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/libs/strings"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	grpccore "github.com/tendermint/tendermint/rpc/grpc"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/types"

	_ "net/http/pprof" // nolint: gosec // securely exposed on separate, optional port

	_ "github.com/lib/pq" // provide the psql db driver
)

// nodeImpl is the highest level interface to a full Tendermint node.
// It includes all configuration information and running services.
type nodeImpl struct {
	service.BaseService

	// config
	config        *config.Config
	genesisDoc    *types.GenesisDoc   // initial validator set
	privValidator types.PrivValidator // local node's validator key

	// network
	transport   *p2p.MConnTransport
	sw          *p2p.Switch // p2p connections
	peerManager *p2p.PeerManager
	router      *p2p.Router
	addrBook    pex.AddrBook // known peers
	nodeInfo    types.NodeInfo
	nodeKey     types.NodeKey // our node privkey
	isListening bool

	// services
	eventBus         *types.EventBus // pub/sub for services
	stateStore       sm.Store
	blockStore       *store.BlockStore // store the blockchain to disk
	bcReactor        service.Service   // for block-syncing
	mempoolReactor   service.Service   // for gossipping transactions
	mempool          mempool.Mempool
	stateSync        bool               // whether the node should state sync on startup
	stateSyncReactor *statesync.Reactor // for hosting and restoring state sync snapshots
	consensusReactor *consensus.Reactor // for participating in the consensus
	pexReactor       service.Service    // for exchanging peer addresses
	evidenceReactor  service.Service
	rpcListeners     []net.Listener // rpc servers
	indexerService   service.Service
	rpcEnv           *rpccore.Environment
	prometheusSrv    *http.Server
}

// newDefaultNode returns a Tendermint node with default settings for the
// PrivValidator, ClientCreator, GenesisDoc, and DBProvider.
// It implements NodeProvider.
func newDefaultNode(cfg *config.Config, logger log.Logger) (service.Service, error) {
	nodeKey, err := types.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load or gen node key %s: %w", cfg.NodeKeyFile(), err)
	}
	if cfg.Mode == config.ModeSeed {
		return makeSeedNode(cfg,
			config.DefaultDBProvider,
			nodeKey,
			defaultGenesisDocProviderFunc(cfg),
			logger,
		)
	}

	var pval *privval.FilePV
	if cfg.Mode == config.ModeValidator {
		pval, err = privval.LoadOrGenFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
		if err != nil {
			return nil, err
		}
	} else {
		pval = nil
	}

	appClient, _ := proxy.DefaultClientCreator(cfg.ProxyApp, cfg.ABCI, cfg.DBDir())
	return makeNode(cfg,
		pval,
		nodeKey,
		appClient,
		defaultGenesisDocProviderFunc(cfg),
		config.DefaultDBProvider,
		logger,
	)
}

// makeNode returns a new, ready to go, Tendermint Node.
func makeNode(cfg *config.Config,
	privValidator types.PrivValidator,
	nodeKey types.NodeKey,
	clientCreator abciclient.Creator,
	genesisDocProvider genesisDocProvider,
	dbProvider config.DBProvider,
	logger log.Logger) (service.Service, error) {

	blockStore, stateDB, err := initDBs(cfg, dbProvider)
	if err != nil {
		return nil, err
	}
	stateStore := sm.NewStore(stateDB)

	genDoc, err := genesisDocProvider()
	if err != nil {
		return nil, err
	}

	err = genDoc.ValidateAndComplete()
	if err != nil {
		return nil, fmt.Errorf("error in genesis doc: %w", err)
	}

	state, err := loadStateFromDBOrGenesisDocProvider(stateStore, genDoc)
	if err != nil {
		return nil, err
	}

	// Create the proxyApp and establish connections to the ABCI app (consensus, mempool, query).
	proxyApp, err := createAndStartProxyAppConns(clientCreator, logger)
	if err != nil {
		return nil, err
	}

	// EventBus and IndexerService must be started before the handshake because
	// we might need to index the txs of the replayed block as this might not have happened
	// when the node stopped last time (i.e. the node stopped after it saved the block
	// but before it indexed the txs, or, endblocker panicked)
	eventBus, err := createAndStartEventBus(logger)
	if err != nil {
		return nil, err
	}

	indexerService, eventSinks, err := createAndStartIndexerService(cfg, dbProvider, eventBus, logger, genDoc.ChainID)
	if err != nil {
		return nil, err
	}

	// If an address is provided, listen on the socket for a connection from an
	// external signing process.
	if cfg.PrivValidator.ListenAddr != "" {
		protocol, _ := tmnet.ProtocolAndAddress(cfg.PrivValidator.ListenAddr)
		// FIXME: we should start services inside OnStart
		switch protocol {
		case "grpc":
			privValidator, err = createAndStartPrivValidatorGRPCClient(cfg, genDoc.ChainID, logger)
			if err != nil {
				return nil, fmt.Errorf("error with private validator grpc client: %w", err)
			}
		default:
			privValidator, err = createAndStartPrivValidatorSocketClient(cfg.PrivValidator.ListenAddr, genDoc.ChainID, logger)
			if err != nil {
				return nil, fmt.Errorf("error with private validator socket client: %w", err)
			}
		}
	}
	var pubKey crypto.PubKey
	if cfg.Mode == config.ModeValidator {
		pubKey, err = privValidator.GetPubKey(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		if pubKey == nil {
			return nil, errors.New("could not retrieve public key from private validator")
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
	consensusLogger := logger.With("module", "consensus")
	if !stateSync {
		if err := doHandshake(stateStore, state, blockStore, genDoc, eventBus, proxyApp, consensusLogger); err != nil {
			return nil, err
		}

		// Reload the state. It will have the Version.Consensus.App set by the
		// Handshake, and may have other modifications as well (ie. depending on
		// what happened during block replay).
		state, err = stateStore.Load()
		if err != nil {
			return nil, fmt.Errorf("cannot load state: %w", err)
		}
	}

	// Determine whether we should do block sync. This must happen after the handshake, since the
	// app may modify the validator set, specifying ourself as the only validator.
	blockSync := cfg.BlockSync.Enable && !onlyValidatorIsUs(state, pubKey)

	logNodeStartupInfo(state, pubKey, logger, consensusLogger, cfg.Mode)

	// TODO: Fetch and provide real options and do proper p2p bootstrapping.
	// TODO: Use a persistent peer database.
	nodeInfo, err := makeNodeInfo(cfg, nodeKey, eventSinks, genDoc, state)
	if err != nil {
		return nil, err
	}

	p2pLogger := logger.With("module", "p2p")
	transport := createTransport(p2pLogger, cfg)

	peerManager, err := createPeerManager(cfg, dbProvider, p2pLogger, nodeKey.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer manager: %w", err)
	}

	nodeMetrics :=
		defaultMetricsProvider(cfg.Instrumentation)(genDoc.ChainID)

	router, err := createRouter(p2pLogger, nodeMetrics.p2p, nodeInfo, nodeKey.PrivKey,
		peerManager, transport, getRouterConfig(cfg, proxyApp))
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	mpReactorShim, mpReactor, mp, err := createMempoolReactor(
		cfg, proxyApp, state, nodeMetrics.mempool, peerManager, router, logger,
	)
	if err != nil {
		return nil, err
	}

	evReactorShim, evReactor, evPool, err := createEvidenceReactor(
		cfg, dbProvider, stateDB, blockStore, peerManager, router, logger,
	)
	if err != nil {
		return nil, err
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

	csReactorShim, csReactor, csState := createConsensusReactor(
		cfg, state, blockExec, blockStore, mp, evPool,
		privValidator, nodeMetrics.consensus, stateSync || blockSync, eventBus,
		peerManager, router, consensusLogger,
	)

	// Create the blockchain reactor. Note, we do not start block sync if we're
	// doing a state sync first.
	bcReactorShim, bcReactor, err := createBlockchainReactor(
		logger, cfg, state, blockExec, blockStore, csReactor,
		peerManager, router, blockSync && !stateSync, nodeMetrics.consensus,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create blockchain reactor: %w", err)
	}

	// TODO: Remove this once the switch is removed.
	var bcReactorForSwitch p2p.Reactor
	if bcReactorShim != nil {
		bcReactorForSwitch = bcReactorShim
	} else {
		bcReactorForSwitch = bcReactor.(p2p.Reactor)
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
	var (
		stateSyncReactor     *statesync.Reactor
		stateSyncReactorShim *p2p.ReactorShim

		channels    map[p2p.ChannelID]*p2p.Channel
		peerUpdates *p2p.PeerUpdates
	)

	stateSyncReactorShim = p2p.NewReactorShim(logger.With("module", "statesync"), "StateSyncShim", statesync.ChannelShims)

	if cfg.P2P.UseLegacy {
		channels = getChannelsFromShim(stateSyncReactorShim)
		peerUpdates = stateSyncReactorShim.PeerUpdates
	} else {
		channels = makeChannelsFromShims(router, statesync.ChannelShims)
		peerUpdates = peerManager.Subscribe()
	}

	stateSyncReactor = statesync.NewReactor(
		genDoc.ChainID,
		genDoc.InitialHeight,
		*cfg.StateSync,
		stateSyncReactorShim.Logger,
		proxyApp.Snapshot(),
		proxyApp.Query(),
		channels[statesync.SnapshotChannel],
		channels[statesync.ChunkChannel],
		channels[statesync.LightBlockChannel],
		channels[statesync.ParamsChannel],
		peerUpdates,
		stateStore,
		blockStore,
		cfg.StateSync.TempDir,
		nodeMetrics.statesync,
	)

	// add the channel descriptors to both the transports
	// FIXME: This should be removed when the legacy p2p stack is removed and
	// transports can either be agnostic to channel descriptors or can be
	// declared in the constructor.
	transport.AddChannelDescriptors(mpReactorShim.GetChannels())
	transport.AddChannelDescriptors(bcReactorForSwitch.GetChannels())
	transport.AddChannelDescriptors(csReactorShim.GetChannels())
	transport.AddChannelDescriptors(evReactorShim.GetChannels())
	transport.AddChannelDescriptors(stateSyncReactorShim.GetChannels())

	// Optionally, start the pex reactor
	//
	// TODO:
	//
	// We need to set Seeds and PersistentPeers on the switch,
	// since it needs to be able to use these (and their DNS names)
	// even if the PEX is off. We can include the DNS name in the NetAddress,
	// but it would still be nice to have a clear list of the current "PersistentPeers"
	// somewhere that we can return with net_info.
	//
	// If PEX is on, it should handle dialing the seeds. Otherwise the switch does it.
	// Note we currently use the addrBook regardless at least for AddOurAddress

	var (
		pexReactor service.Service
		sw         *p2p.Switch
		addrBook   pex.AddrBook
	)

	pexCh := pex.ChannelDescriptor()
	transport.AddChannelDescriptors([]*p2p.ChannelDescriptor{&pexCh})

	if cfg.P2P.UseLegacy {
		// setup Transport and Switch
		sw = createSwitch(
			cfg, transport, nodeMetrics.p2p, mpReactorShim, bcReactorForSwitch,
			stateSyncReactorShim, csReactorShim, evReactorShim, proxyApp, nodeInfo, nodeKey, p2pLogger,
		)

		err = sw.AddPersistentPeers(strings.SplitAndTrimEmpty(cfg.P2P.PersistentPeers, ",", " "))
		if err != nil {
			return nil, fmt.Errorf("could not add peers from persistent-peers field: %w", err)
		}

		err = sw.AddUnconditionalPeerIDs(strings.SplitAndTrimEmpty(cfg.P2P.UnconditionalPeerIDs, ",", " "))
		if err != nil {
			return nil, fmt.Errorf("could not add peer ids from unconditional_peer_ids field: %w", err)
		}

		addrBook, err = createAddrBookAndSetOnSwitch(cfg, sw, p2pLogger, nodeKey)
		if err != nil {
			return nil, fmt.Errorf("could not create addrbook: %w", err)
		}

		if cfg.P2P.PexReactor {
			pexReactor = createPEXReactorAndAddToSwitch(addrBook, cfg, sw, logger)
		}
	} else {
		addrBook = nil
		if cfg.P2P.PexReactor {
			pexReactor, err = createPEXReactorV2(cfg, logger, peerManager, router)
			if err != nil {
				return nil, err
			}
		}
	}
	if cfg.RPC.PprofListenAddress != "" {
		go func() {
			logger.Info("Starting pprof server", "laddr", cfg.RPC.PprofListenAddress)
			logger.Error("pprof server error", "err", http.ListenAndServe(cfg.RPC.PprofListenAddress, nil))
		}()
	}

	node := &nodeImpl{
		config:        cfg,
		genesisDoc:    genDoc,
		privValidator: privValidator,

		transport:   transport,
		sw:          sw,
		peerManager: peerManager,
		router:      router,
		addrBook:    addrBook,
		nodeInfo:    nodeInfo,
		nodeKey:     nodeKey,

		stateStore:       stateStore,
		blockStore:       blockStore,
		bcReactor:        bcReactor,
		mempoolReactor:   mpReactor,
		mempool:          mp,
		consensusReactor: csReactor,
		stateSyncReactor: stateSyncReactor,
		stateSync:        stateSync,
		pexReactor:       pexReactor,
		evidenceReactor:  evReactor,
		indexerService:   indexerService,
		eventBus:         eventBus,

		rpcEnv: &rpccore.Environment{
			ProxyAppQuery:   proxyApp.Query(),
			ProxyAppMempool: proxyApp.Mempool(),

			StateStore:     stateStore,
			BlockStore:     blockStore,
			EvidencePool:   evPool,
			ConsensusState: csState,

			ConsensusReactor: csReactor,
			BlockSyncReactor: bcReactor.(consensus.BlockSyncReactor),

			P2PPeers:    sw,
			PeerManager: peerManager,

			GenDoc:     genDoc,
			EventSinks: eventSinks,
			EventBus:   eventBus,
			Mempool:    mp,
			Logger:     logger.With("module", "rpc"),
			Config:     *cfg.RPC,
		},
	}

	// this is a terrible, because typed nil interfaces are not ==
	// nil, so this is just cleanup to avoid having a non-nil
	// value in the RPC environment that has the semantic
	// properties of nil.
	if sw == nil {
		node.rpcEnv.P2PPeers = nil
	} else if peerManager == nil {
		node.rpcEnv.PeerManager = nil
	}
	// end hack

	node.rpcEnv.P2PTransport = node

	node.BaseService = *service.NewBaseService(logger, "Node", node)

	return node, nil
}

// makeSeedNode returns a new seed node, containing only p2p, pex reactor
func makeSeedNode(cfg *config.Config,
	dbProvider config.DBProvider,
	nodeKey types.NodeKey,
	genesisDocProvider genesisDocProvider,
	logger log.Logger,
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
	p2pLogger := logger.With("module", "p2p")
	transport := createTransport(p2pLogger, cfg)

	peerManager, err := createPeerManager(cfg, dbProvider, p2pLogger, nodeKey.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer manager: %w", err)
	}

	router, err := createRouter(p2pLogger, p2pMetrics, nodeInfo, nodeKey.PrivKey,
		peerManager, transport, getRouterConfig(cfg, nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	var (
		pexReactor service.Service
		sw         *p2p.Switch
		addrBook   pex.AddrBook
	)

	// add the pex reactor
	// FIXME: we add channel descriptors to both the router and the transport but only the router
	// should be aware of channel info. We should remove this from transport once the legacy
	// p2p stack is removed.
	pexCh := pex.ChannelDescriptor()
	transport.AddChannelDescriptors([]*p2p.ChannelDescriptor{&pexCh})

	if cfg.P2P.UseLegacy {
		sw = createSwitch(
			cfg, transport, p2pMetrics, nil, nil,
			nil, nil, nil, nil, nodeInfo, nodeKey, p2pLogger,
		)

		err = sw.AddPersistentPeers(strings.SplitAndTrimEmpty(cfg.P2P.PersistentPeers, ",", " "))
		if err != nil {
			return nil, fmt.Errorf("could not add peers from persistent_peers field: %w", err)
		}

		err = sw.AddUnconditionalPeerIDs(strings.SplitAndTrimEmpty(cfg.P2P.UnconditionalPeerIDs, ",", " "))
		if err != nil {
			return nil, fmt.Errorf("could not add peer ids from unconditional_peer_ids field: %w", err)
		}

		addrBook, err = createAddrBookAndSetOnSwitch(cfg, sw, p2pLogger, nodeKey)
		if err != nil {
			return nil, fmt.Errorf("could not create addrbook: %w", err)
		}

		if cfg.P2P.PexReactor {
			pexReactor = createPEXReactorAndAddToSwitch(addrBook, cfg, sw, logger)
		}
	} else {
		if cfg.P2P.PexReactor {
			pexReactor, err = createPEXReactorV2(cfg, logger, peerManager, router)
			if err != nil {
				return nil, err
			}
		}
	}

	if cfg.RPC.PprofListenAddress != "" {
		go func() {
			logger.Info("Starting pprof server", "laddr", cfg.RPC.PprofListenAddress)
			logger.Error("pprof server error", "err", http.ListenAndServe(cfg.RPC.PprofListenAddress, nil))
		}()
	}

	node := &nodeImpl{
		config:     cfg,
		genesisDoc: genDoc,

		transport:   transport,
		sw:          sw,
		addrBook:    addrBook,
		nodeInfo:    nodeInfo,
		nodeKey:     nodeKey,
		peerManager: peerManager,
		router:      router,

		pexReactor: pexReactor,
	}
	node.BaseService = *service.NewBaseService(logger, "SeedNode", node)

	return node, nil
}

// OnStart starts the Node. It implements service.Service.
func (n *nodeImpl) OnStart() error {
	now := tmtime.Now()
	genTime := n.genesisDoc.GenesisTime
	if genTime.After(now) {
		n.Logger.Info("Genesis time is in the future. Sleeping until then...", "genTime", genTime)
		time.Sleep(genTime.Sub(now))
	}

	// Start the RPC server before the P2P server
	// so we can eg. receive txs for the first block
	if n.config.RPC.ListenAddress != "" && n.config.Mode != config.ModeSeed {
		listeners, err := n.startRPC()
		if err != nil {
			return err
		}
		n.rpcListeners = listeners
	}

	if n.config.Instrumentation.Prometheus &&
		n.config.Instrumentation.PrometheusListenAddr != "" {
		n.prometheusSrv = n.startPrometheusServer(n.config.Instrumentation.PrometheusListenAddr)
	}

	// Start the transport.
	addr, err := types.NewNetAddressString(n.nodeKey.ID.AddressString(n.config.P2P.ListenAddress))
	if err != nil {
		return err
	}
	if err := n.transport.Listen(p2p.NewEndpoint(addr)); err != nil {
		return err
	}

	n.isListening = true
	n.Logger.Info("p2p service", "legacy_enabled", n.config.P2P.UseLegacy)

	if n.config.P2P.UseLegacy {
		// Add private IDs to addrbook to block those peers being added
		n.addrBook.AddPrivateIDs(strings.SplitAndTrimEmpty(n.config.P2P.PrivatePeerIDs, ",", " "))
		if err = n.sw.Start(); err != nil {
			return err
		}
	} else if err = n.router.Start(); err != nil {
		return err
	}

	if n.config.Mode != config.ModeSeed {
		if n.config.BlockSync.Version == config.BlockSyncV0 {
			if err := n.bcReactor.Start(); err != nil {
				return err
			}
		}

		// Start the real consensus reactor separately since the switch uses the shim.
		if err := n.consensusReactor.Start(); err != nil {
			return err
		}

		// Start the real state sync reactor separately since the switch uses the shim.
		if err := n.stateSyncReactor.Start(); err != nil {
			return err
		}

		// Start the real mempool reactor separately since the switch uses the shim.
		if err := n.mempoolReactor.Start(); err != nil {
			return err
		}

		// Start the real evidence reactor separately since the switch uses the shim.
		if err := n.evidenceReactor.Start(); err != nil {
			return err
		}
	}

	if n.config.P2P.UseLegacy {
		// Always connect to persistent peers
		err = n.sw.DialPeersAsync(strings.SplitAndTrimEmpty(n.config.P2P.PersistentPeers, ",", " "))
		if err != nil {
			return fmt.Errorf("could not dial peers from persistent-peers field: %w", err)
		}
	} else if n.config.P2P.PexReactor {
		if err := n.pexReactor.Start(); err != nil {
			return err
		}
	}

	// Run state sync
	// TODO: We shouldn't run state sync if we already have state that has a
	// LastBlockHeight that is not InitialHeight
	if n.stateSync {
		bcR, ok := n.bcReactor.(consensus.BlockSyncReactor)
		if !ok {
			return fmt.Errorf("this blockchain reactor does not support switching from state sync")
		}

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
		if err := n.eventBus.PublishEventStateSyncStatus(d); err != nil {
			n.eventBus.Logger.Error("failed to emit the statesync start event", "err", err)
		}

		// FIXME: We shouldn't allow state sync to silently error out without
		// bubbling up the error and gracefully shutting down the rest of the node
		go func() {
			n.Logger.Info("starting state sync")
			state, err := n.stateSyncReactor.Sync(context.TODO())
			if err != nil {
				n.Logger.Error("state sync failed; shutting down this node", "err", err)
				// stop the node
				if err := n.Stop(); err != nil {
					n.Logger.Error("failed to shut down node", "err", err)
				}
				return
			}

			n.consensusReactor.SetStateSyncingMetrics(0)

			d := types.EventDataStateSyncStatus{Complete: true, Height: state.LastBlockHeight}
			if err := n.eventBus.PublishEventStateSyncStatus(d); err != nil {
				n.eventBus.Logger.Error("failed to emit the statesync start event", "err", err)
			}

			// TODO: Some form of orchestrator is needed here between the state
			// advancing reactors to be able to control which one of the three
			// is running
			if n.config.BlockSync.Enable {
				// FIXME Very ugly to have these metrics bleed through here.
				n.consensusReactor.SetBlockSyncingMetrics(1)
				if err := bcR.SwitchToBlockSync(state); err != nil {
					n.Logger.Error("failed to switch to block sync", "err", err)
					return
				}

				d := types.EventDataBlockSyncStatus{Complete: false, Height: state.LastBlockHeight}
				if err := n.eventBus.PublishEventBlockSyncStatus(d); err != nil {
					n.eventBus.Logger.Error("failed to emit the block sync starting event", "err", err)
				}

			} else {
				n.consensusReactor.SwitchToConsensus(state, true)
			}
		}()
	}

	return nil
}

// OnStop stops the Node. It implements service.Service.
func (n *nodeImpl) OnStop() {

	n.Logger.Info("Stopping Node")

	// first stop the non-reactor services
	if err := n.eventBus.Stop(); err != nil {
		n.Logger.Error("Error closing eventBus", "err", err)
	}
	if err := n.indexerService.Stop(); err != nil {
		n.Logger.Error("Error closing indexerService", "err", err)
	}

	if n.config.Mode != config.ModeSeed {
		// now stop the reactors
		if n.config.BlockSync.Version == config.BlockSyncV0 {
			// Stop the real blockchain reactor separately since the switch uses the shim.
			if err := n.bcReactor.Stop(); err != nil {
				n.Logger.Error("failed to stop the blockchain reactor", "err", err)
			}
		}

		// Stop the real consensus reactor separately since the switch uses the shim.
		if err := n.consensusReactor.Stop(); err != nil {
			n.Logger.Error("failed to stop the consensus reactor", "err", err)
		}

		// Stop the real state sync reactor separately since the switch uses the shim.
		if err := n.stateSyncReactor.Stop(); err != nil {
			n.Logger.Error("failed to stop the state sync reactor", "err", err)
		}

		// Stop the real mempool reactor separately since the switch uses the shim.
		if err := n.mempoolReactor.Stop(); err != nil {
			n.Logger.Error("failed to stop the mempool reactor", "err", err)
		}

		// Stop the real evidence reactor separately since the switch uses the shim.
		if err := n.evidenceReactor.Stop(); err != nil {
			n.Logger.Error("failed to stop the evidence reactor", "err", err)
		}
	}

	if err := n.pexReactor.Stop(); err != nil {
		n.Logger.Error("failed to stop the PEX v2 reactor", "err", err)
	}

	if n.config.P2P.UseLegacy {
		if err := n.sw.Stop(); err != nil {
			n.Logger.Error("failed to stop switch", "err", err)
		}
	} else {
		if err := n.router.Stop(); err != nil {
			n.Logger.Error("failed to stop router", "err", err)
		}
	}

	if err := n.transport.Close(); err != nil {
		n.Logger.Error("Error closing transport", "err", err)
	}

	n.isListening = false

	// finally stop the listeners / external services
	for _, l := range n.rpcListeners {
		n.Logger.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			n.Logger.Error("Error closing listener", "listener", l, "err", err)
		}
	}

	if pvsc, ok := n.privValidator.(service.Service); ok {
		if err := pvsc.Stop(); err != nil {
			n.Logger.Error("Error closing private validator", "err", err)
		}
	}

	if n.prometheusSrv != nil {
		if err := n.prometheusSrv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			n.Logger.Error("Prometheus HTTP server Shutdown", "err", err)
		}
	}
}

func (n *nodeImpl) startRPC() ([]net.Listener, error) {
	if n.config.Mode == config.ModeValidator {
		pubKey, err := n.privValidator.GetPubKey(context.TODO())
		if pubKey == nil || err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		n.rpcEnv.PubKey = pubKey
	}
	if err := n.rpcEnv.InitGenesisChunks(); err != nil {
		return nil, err
	}

	listenAddrs := strings.SplitAndTrimEmpty(n.config.RPC.ListenAddress, ",", " ")
	routes := n.rpcEnv.GetRoutes()

	if n.config.RPC.Unsafe {
		n.rpcEnv.AddUnsafe(routes)
	}

	cfg := rpcserver.DefaultConfig()
	cfg.MaxBodyBytes = n.config.RPC.MaxBodyBytes
	cfg.MaxHeaderBytes = n.config.RPC.MaxHeaderBytes
	cfg.MaxOpenConnections = n.config.RPC.MaxOpenConnections
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	if cfg.WriteTimeout <= n.config.RPC.TimeoutBroadcastTxCommit {
		cfg.WriteTimeout = n.config.RPC.TimeoutBroadcastTxCommit + 1*time.Second
	}

	// we may expose the rpc over both a unix and tcp socket
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		rpcLogger := n.Logger.With("module", "rpc-server")
		wmLogger := rpcLogger.With("protocol", "websocket")
		wm := rpcserver.NewWebsocketManager(routes,
			rpcserver.OnDisconnect(func(remoteAddr string) {
				err := n.eventBus.UnsubscribeAll(context.Background(), remoteAddr)
				if err != nil && err != tmpubsub.ErrSubscriptionNotFound {
					wmLogger.Error("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
				}
			}),
			rpcserver.ReadLimit(cfg.MaxBodyBytes),
		)
		wm.SetLogger(wmLogger)
		mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(mux, routes, rpcLogger)
		listener, err := rpcserver.Listen(
			listenAddr,
			cfg.MaxOpenConnections,
		)
		if err != nil {
			return nil, err
		}

		var rootHandler http.Handler = mux
		if n.config.RPC.IsCorsEnabled() {
			corsMiddleware := cors.New(cors.Options{
				AllowedOrigins: n.config.RPC.CORSAllowedOrigins,
				AllowedMethods: n.config.RPC.CORSAllowedMethods,
				AllowedHeaders: n.config.RPC.CORSAllowedHeaders,
			})
			rootHandler = corsMiddleware.Handler(mux)
		}
		if n.config.RPC.IsTLSEnabled() {
			go func() {
				if err := rpcserver.ServeTLS(
					listener,
					rootHandler,
					n.config.RPC.CertFile(),
					n.config.RPC.KeyFile(),
					rpcLogger,
					cfg,
				); err != nil {
					n.Logger.Error("Error serving server with TLS", "err", err)
				}
			}()
		} else {
			go func() {
				if err := rpcserver.Serve(
					listener,
					rootHandler,
					rpcLogger,
					cfg,
				); err != nil {
					n.Logger.Error("Error serving server", "err", err)
				}
			}()
		}

		listeners[i] = listener
	}

	// we expose a simplified api over grpc for convenience to app devs
	grpcListenAddr := n.config.RPC.GRPCListenAddress
	if grpcListenAddr != "" {
		cfg := rpcserver.DefaultConfig()
		cfg.MaxBodyBytes = n.config.RPC.MaxBodyBytes
		cfg.MaxHeaderBytes = n.config.RPC.MaxHeaderBytes
		// NOTE: GRPCMaxOpenConnections is used, not MaxOpenConnections
		cfg.MaxOpenConnections = n.config.RPC.GRPCMaxOpenConnections
		// If necessary adjust global WriteTimeout to ensure it's greater than
		// TimeoutBroadcastTxCommit.
		// See https://github.com/tendermint/tendermint/issues/3435
		if cfg.WriteTimeout <= n.config.RPC.TimeoutBroadcastTxCommit {
			cfg.WriteTimeout = n.config.RPC.TimeoutBroadcastTxCommit + 1*time.Second
		}
		listener, err := rpcserver.Listen(grpcListenAddr, cfg.MaxOpenConnections)
		if err != nil {
			return nil, err
		}
		go func() {
			if err := grpccore.StartGRPCServer(n.rpcEnv, listener); err != nil {
				n.Logger.Error("Error starting gRPC server", "err", err)
			}
		}()
		listeners = append(listeners, listener)

	}

	return listeners, nil

}

// startPrometheusServer starts a Prometheus HTTP server, listening for metrics
// collectors on addr.
func (n *nodeImpl) startPrometheusServer(addr string) *http.Server {
	srv := &http.Server{
		Addr: addr,
		Handler: promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{MaxRequestsInFlight: n.config.Instrumentation.MaxOpenConnections},
			),
		),
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			n.Logger.Error("Prometheus HTTP server ListenAndServe", "err", err)
		}
	}()
	return srv
}

// ConsensusReactor returns the Node's ConsensusReactor.
func (n *nodeImpl) ConsensusReactor() *consensus.Reactor {
	return n.consensusReactor
}

// Mempool returns the Node's mempool.
func (n *nodeImpl) Mempool() mempool.Mempool {
	return n.mempool
}

// EventBus returns the Node's EventBus.
func (n *nodeImpl) EventBus() *types.EventBus {
	return n.eventBus
}

// PrivValidator returns the Node's PrivValidator.
// XXX: for convenience only!
func (n *nodeImpl) PrivValidator() types.PrivValidator {
	return n.privValidator
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
	p2p       *p2p.Metrics
	mempool   *mempool.Metrics
	state     *sm.Metrics
	statesync *statesync.Metrics
}

// metricsProvider returns consensus, p2p, mempool, state, statesync Metrics.
type metricsProvider func(chainID string) *nodeMetrics

// defaultMetricsProvider returns Metrics build using Prometheus client library
// if Prometheus is enabled. Otherwise, it returns no-op Metrics.
func defaultMetricsProvider(cfg *config.InstrumentationConfig) metricsProvider {
	return func(chainID string) *nodeMetrics {
		if cfg.Prometheus {
			return &nodeMetrics{
				consensus.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				p2p.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				mempool.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				sm.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
				statesync.PrometheusMetrics(cfg.Namespace, "chain_id", chainID),
			}
		}
		return &nodeMetrics{
			consensus.NopMetrics(),
			p2p.NopMetrics(),
			mempool.NopMetrics(),
			sm.NopMetrics(),
			statesync.NopMetrics(),
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

func createAndStartPrivValidatorSocketClient(
	listenAddr,
	chainID string,
	logger log.Logger,
) (types.PrivValidator, error) {

	pve, err := privval.NewSignerListener(listenAddr, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start private validator: %w", err)
	}

	pvsc, err := privval.NewSignerClient(pve, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to start private validator: %w", err)
	}

	// try to get a pubkey from private validate first time
	_, err = pvsc.GetPubKey(context.TODO())
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

func createAndStartPrivValidatorGRPCClient(
	cfg *config.Config,
	chainID string,
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
	_, err = pvsc.GetPubKey(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}

	return pvsc, nil
}

func getRouterConfig(conf *config.Config, proxyApp proxy.AppConns) p2p.RouterOptions {
	opts := p2p.RouterOptions{
		QueueType: conf.P2P.QueueType,
	}

	if conf.P2P.MaxNumInboundPeers > 0 {
		opts.MaxIncomingConnectionAttempts = conf.P2P.MaxIncomingConnectionAttempts
	}

	if conf.FilterPeers && proxyApp != nil {
		opts.FilterPeerByID = func(ctx context.Context, id types.NodeID) error {
			res, err := proxyApp.Query().QuerySync(context.Background(), abci.RequestQuery{
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
			res, err := proxyApp.Query().QuerySync(ctx, abci.RequestQuery{
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

// FIXME: Temporary helper function, shims should be removed.
func makeChannelsFromShims(
	router *p2p.Router,
	chShims map[p2p.ChannelID]*p2p.ChannelDescriptorShim,
) map[p2p.ChannelID]*p2p.Channel {

	channels := map[p2p.ChannelID]*p2p.Channel{}
	for chID, chShim := range chShims {
		ch, err := router.OpenChannel(*chShim.Descriptor, chShim.MsgType, chShim.Descriptor.RecvBufferCapacity)
		if err != nil {
			panic(fmt.Sprintf("failed to open channel %v: %v", chID, err))
		}

		channels[chID] = ch
	}

	return channels
}

func getChannelsFromShim(reactorShim *p2p.ReactorShim) map[p2p.ChannelID]*p2p.Channel {
	channels := map[p2p.ChannelID]*p2p.Channel{}
	for chID := range reactorShim.Channels {
		channels[chID] = reactorShim.GetChannel(chID)
	}

	return channels
}
