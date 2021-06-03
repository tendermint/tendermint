package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // nolint: gosec // securely exposed on separate, optional port
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/evidence"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/libs/strings"
	"github.com/tendermint/tendermint/light"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/pex"
	"github.com/tendermint/tendermint/privval"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	"github.com/tendermint/tendermint/proxy"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	grpccore "github.com/tendermint/tendermint/rpc/grpc"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/statesync"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// Node is the highest level interface to a full Tendermint node.
// It includes all configuration information and running services.
type Node struct {
	service.BaseService

	// config
	config        *cfg.Config
	genesisDoc    *types.GenesisDoc   // initial validator set
	privValidator types.PrivValidator // local node's validator key

	// network
	transport   *p2p.MConnTransport
	sw          *p2p.Switch // p2p connections
	peerManager *p2p.PeerManager
	router      *p2p.Router
	addrBook    pex.AddrBook // known peers
	nodeInfo    p2p.NodeInfo
	nodeKey     p2p.NodeKey // our node privkey
	isListening bool

	// services
	eventBus          *types.EventBus // pub/sub for services
	stateStore        sm.Store
	blockStore        *store.BlockStore // store the blockchain to disk
	bcReactor         service.Service   // for fast-syncing
	mempoolReactor    *mempl.Reactor    // for gossipping transactions
	mempool           mempl.Mempool
	stateSync         bool                    // whether the node should state sync on startup
	stateSyncReactor  *statesync.Reactor      // for hosting and restoring state sync snapshots
	stateSyncProvider statesync.StateProvider // provides state data for bootstrapping a node
	stateSyncGenesis  sm.State                // provides the genesis state for state sync
	consensusState    *cs.State               // latest consensus state
	consensusReactor  *cs.Reactor             // for participating in the consensus
	pexReactor        *pex.Reactor            // for exchanging peer addresses
	pexReactorV2      *pex.ReactorV2          // for exchanging peer addresses
	evidenceReactor   *evidence.Reactor
	evidencePool      *evidence.Pool // tracking evidence
	proxyApp          proxy.AppConns // connection to the application
	rpcListeners      []net.Listener // rpc servers
	txIndexer         indexer.TxIndexer
	blockIndexer      indexer.BlockIndexer
	indexerService    *indexer.Service
	prometheusSrv     *http.Server
}

// DefaultNewNode returns a Tendermint node with default settings for the
// PrivValidator, ClientCreator, GenesisDoc, and DBProvider.
// It implements NodeProvider.
func DefaultNewNode(config *cfg.Config, logger log.Logger) (*Node, error) {
	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load or gen node key %s: %w", config.NodeKeyFile(), err)
	}
	if config.Mode == cfg.ModeSeed {
		return NewSeedNode(config,
			DefaultDBProvider,
			nodeKey,
			DefaultGenesisDocProviderFunc(config),
			logger,
		)
	}

	var pval *privval.FilePV
	if config.Mode == cfg.ModeValidator {
		pval, err = privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
		if err != nil {
			return nil, err
		}
	} else {
		pval = nil
	}

	appClient, _ := proxy.DefaultClientCreator(config.ProxyApp, config.ABCI, config.DBDir())
	return NewNode(config,
		pval,
		nodeKey,
		appClient,
		DefaultGenesisDocProviderFunc(config),
		DefaultDBProvider,
		DefaultMetricsProvider(config.Instrumentation),
		logger,
	)
}

// NewNode returns a new, ready to go, Tendermint Node.
func NewNode(config *cfg.Config,
	privValidator types.PrivValidator,
	nodeKey p2p.NodeKey,
	clientCreator proxy.ClientCreator,
	genesisDocProvider GenesisDocProvider,
	dbProvider DBProvider,
	metricsProvider MetricsProvider,
	logger log.Logger,
	options ...Option) (*Node, error) {

	blockStore, stateDB, err := initDBs(config, dbProvider)
	if err != nil {
		return nil, err
	}

	stateStore := sm.NewStore(stateDB)

	state, genDoc, err := LoadStateFromDBOrGenesisDocProvider(stateDB, genesisDocProvider)
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

	indexerService, txIndexer, blockIndexer, err := createAndStartIndexerService(config, dbProvider, eventBus, logger)
	if err != nil {
		return nil, err
	}

	// If an address is provided, listen on the socket for a connection from an
	// external signing process.
	if config.PrivValidatorListenAddr != "" {
		protocol, _ := tmnet.ProtocolAndAddress(config.PrivValidatorListenAddr)
		// FIXME: we should start services inside OnStart
		switch protocol {
		case "grpc":
			privValidator, err = createAndStartPrivValidatorGRPCClient(config, genDoc.ChainID, logger)
			if err != nil {
				return nil, fmt.Errorf("error with private validator grpc client: %w", err)
			}
		default:
			privValidator, err = createAndStartPrivValidatorSocketClient(config.PrivValidatorListenAddr, genDoc.ChainID, logger)
			if err != nil {
				return nil, fmt.Errorf("error with private validator socket client: %w", err)
			}
		}
	}
	var pubKey crypto.PubKey
	if config.Mode == cfg.ModeValidator {
		pubKey, err = privValidator.GetPubKey(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		if pubKey == nil {
			return nil, errors.New("could not retrieve public key from private validator")
		}
	}

	// Determine whether we should attempt state sync.
	stateSync := config.StateSync.Enable && !onlyValidatorIsUs(state, pubKey)
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

	// Determine whether we should do fast sync. This must happen after the handshake, since the
	// app may modify the validator set, specifying ourself as the only validator.
	fastSync := config.FastSyncMode && !onlyValidatorIsUs(state, pubKey)

	logNodeStartupInfo(state, pubKey, logger, consensusLogger, config.Mode)

	// TODO: Fetch and provide real options and do proper p2p bootstrapping.
	// TODO: Use a persistent peer database.
	nodeInfo, err := makeNodeInfo(config, nodeKey, txIndexer, genDoc, state)
	if err != nil {
		return nil, err
	}

	p2pLogger := logger.With("module", "p2p")
	transport := createTransport(p2pLogger, config)

	peerManager, err := createPeerManager(config, dbProvider, p2pLogger, nodeKey.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer manager: %w", err)
	}

	csMetrics, p2pMetrics, memplMetrics, smMetrics := metricsProvider(genDoc.ChainID)

	router, err := createRouter(p2pLogger, p2pMetrics, nodeInfo, nodeKey.PrivKey,
		peerManager, transport, getRouterConfig(config, proxyApp))
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	mpReactorShim, mpReactor, mempool := createMempoolReactor(
		config, proxyApp, state, memplMetrics, peerManager, router, logger,
	)

	evReactorShim, evReactor, evPool, err := createEvidenceReactor(
		config, dbProvider, stateDB, blockStore, peerManager, router, logger,
	)
	if err != nil {
		return nil, err
	}

	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger.With("module", "state"),
		proxyApp.Consensus(),
		mempool,
		evPool,
		sm.BlockExecutorWithMetrics(smMetrics),
	)

	csReactorShim, csReactor, csState := createConsensusReactor(
		config, state, blockExec, blockStore, mempool, evPool,
		privValidator, csMetrics, stateSync || fastSync, eventBus,
		peerManager, router, consensusLogger,
	)

	// Create the blockchain reactor. Note, we do not start fast sync if we're
	// doing a state sync first.
	bcReactorShim, bcReactor, err := createBlockchainReactor(
		logger, config, state, blockExec, blockStore, csReactor,
		peerManager, router, fastSync && !stateSync,
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

	// Make ConsensusReactor. Don't enable fully if doing a state sync and/or fast sync first.
	// FIXME We need to update metrics here, since other reactors don't have access to them.
	if stateSync {
		csMetrics.StateSyncing.Set(1)
	} else if fastSync {
		csMetrics.FastSyncing.Set(1)
	}

	// Set up state sync reactor, and schedule a sync if requested.
	// FIXME The way we do phased startups (e.g. replay -> fast sync -> consensus) is very messy,
	// we should clean this whole thing up. See:
	// https://github.com/tendermint/tendermint/issues/4644
	var (
		stateSyncReactor     *statesync.Reactor
		stateSyncReactorShim *p2p.ReactorShim

		channels    map[p2p.ChannelID]*p2p.Channel
		peerUpdates *p2p.PeerUpdates
	)

	stateSyncReactorShim = p2p.NewReactorShim(logger.With("module", "statesync"), "StateSyncShim", statesync.ChannelShims)

	if config.P2P.DisableLegacy {
		channels = makeChannelsFromShims(router, statesync.ChannelShims)
		peerUpdates = peerManager.Subscribe()
	} else {
		channels = getChannelsFromShim(stateSyncReactorShim)
		peerUpdates = stateSyncReactorShim.PeerUpdates
	}

	stateSyncReactor = statesync.NewReactor(
		stateSyncReactorShim.Logger,
		proxyApp.Snapshot(),
		proxyApp.Query(),
		channels[statesync.SnapshotChannel],
		channels[statesync.ChunkChannel],
		channels[statesync.LightBlockChannel],
		peerUpdates,
		stateStore,
		blockStore,
		config.StateSync.TempDir,
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
		pexReactor   *pex.Reactor
		pexReactorV2 *pex.ReactorV2
		sw           *p2p.Switch
		addrBook     pex.AddrBook
	)

	pexCh := pex.ChannelDescriptor()
	transport.AddChannelDescriptors([]*p2p.ChannelDescriptor{&pexCh})

	if config.P2P.DisableLegacy {
		addrBook = nil
		pexReactorV2, err = createPEXReactorV2(config, logger, peerManager, router)
		if err != nil {
			return nil, err
		}
	} else {
		// setup Transport and Switch
		sw = createSwitch(
			config, transport, p2pMetrics, mpReactorShim, bcReactorForSwitch,
			stateSyncReactorShim, csReactorShim, evReactorShim, proxyApp, nodeInfo, nodeKey, p2pLogger,
		)

		err = sw.AddPersistentPeers(strings.SplitAndTrimEmpty(config.P2P.PersistentPeers, ",", " "))
		if err != nil {
			return nil, fmt.Errorf("could not add peers from persistent-peers field: %w", err)
		}

		err = sw.AddUnconditionalPeerIDs(strings.SplitAndTrimEmpty(config.P2P.UnconditionalPeerIDs, ",", " "))
		if err != nil {
			return nil, fmt.Errorf("could not add peer ids from unconditional_peer_ids field: %w", err)
		}

		addrBook, err = createAddrBookAndSetOnSwitch(config, sw, p2pLogger, nodeKey)
		if err != nil {
			return nil, fmt.Errorf("could not create addrbook: %w", err)
		}

		pexReactor = createPEXReactorAndAddToSwitch(addrBook, config, sw, logger)
	}

	if config.RPC.PprofListenAddress != "" {
		go func() {
			logger.Info("Starting pprof server", "laddr", config.RPC.PprofListenAddress)
			logger.Error("pprof server error", "err", http.ListenAndServe(config.RPC.PprofListenAddress, nil))
		}()
	}

	node := &Node{
		config:        config,
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
		mempool:          mempool,
		consensusState:   csState,
		consensusReactor: csReactor,
		stateSyncReactor: stateSyncReactor,
		stateSync:        stateSync,
		stateSyncGenesis: state, // Shouldn't be necessary, but need a way to pass the genesis state
		pexReactor:       pexReactor,
		pexReactorV2:     pexReactorV2,
		evidenceReactor:  evReactor,
		evidencePool:     evPool,
		proxyApp:         proxyApp,
		txIndexer:        txIndexer,
		indexerService:   indexerService,
		blockIndexer:     blockIndexer,
		eventBus:         eventBus,
	}
	node.BaseService = *service.NewBaseService(logger, "Node", node)

	for _, option := range options {
		option(node)
	}

	return node, nil
}

// NewSeedNode returns a new seed node, containing only p2p, pex reactor
func NewSeedNode(config *cfg.Config,
	dbProvider DBProvider,
	nodeKey p2p.NodeKey,
	genesisDocProvider GenesisDocProvider,
	logger log.Logger,
	options ...Option) (*Node, error) {

	genDoc, err := genesisDocProvider()
	if err != nil {
		return nil, err
	}

	state, err := sm.MakeGenesisState(genDoc)
	if err != nil {
		return nil, err
	}

	nodeInfo, err := makeSeedNodeInfo(config, nodeKey, genDoc, state)
	if err != nil {
		return nil, err
	}

	// Setup Transport and Switch.
	p2pMetrics := p2p.PrometheusMetrics(config.Instrumentation.Namespace, "chain_id", genDoc.ChainID)
	p2pLogger := logger.With("module", "p2p")
	transport := createTransport(p2pLogger, config)
	sw := createSwitch(
		config, transport, p2pMetrics, nil, nil,
		nil, nil, nil, nil, nodeInfo, nodeKey, p2pLogger,
	)

	err = sw.AddPersistentPeers(strings.SplitAndTrimEmpty(config.P2P.PersistentPeers, ",", " "))
	if err != nil {
		return nil, fmt.Errorf("could not add peers from persistent_peers field: %w", err)
	}

	err = sw.AddUnconditionalPeerIDs(strings.SplitAndTrimEmpty(config.P2P.UnconditionalPeerIDs, ",", " "))
	if err != nil {
		return nil, fmt.Errorf("could not add peer ids from unconditional_peer_ids field: %w", err)
	}

	addrBook, err := createAddrBookAndSetOnSwitch(config, sw, p2pLogger, nodeKey)
	if err != nil {
		return nil, fmt.Errorf("could not create addrbook: %w", err)
	}

	peerManager, err := createPeerManager(config, dbProvider, p2pLogger, nodeKey.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer manager: %w", err)
	}

	router, err := createRouter(p2pLogger, p2pMetrics, nodeInfo, nodeKey.PrivKey,
		peerManager, transport, getRouterConfig(config, nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	var (
		pexReactor   *pex.Reactor
		pexReactorV2 *pex.ReactorV2
	)

	// add the pex reactor
	// FIXME: we add channel descriptors to both the router and the transport but only the router
	// should be aware of channel info. We should remove this from transport once the legacy
	// p2p stack is removed.
	pexCh := pex.ChannelDescriptor()
	transport.AddChannelDescriptors([]*p2p.ChannelDescriptor{&pexCh})
	if config.P2P.DisableLegacy {
		pexReactorV2, err = createPEXReactorV2(config, logger, peerManager, router)
		if err != nil {
			return nil, err
		}
	} else {
		pexReactor = createPEXReactorAndAddToSwitch(addrBook, config, sw, logger)
	}

	if config.RPC.PprofListenAddress != "" {
		go func() {
			logger.Info("Starting pprof server", "laddr", config.RPC.PprofListenAddress)
			logger.Error("pprof server error", "err", http.ListenAndServe(config.RPC.PprofListenAddress, nil))
		}()
	}

	node := &Node{
		config:     config,
		genesisDoc: genDoc,

		transport:   transport,
		sw:          sw,
		addrBook:    addrBook,
		nodeInfo:    nodeInfo,
		nodeKey:     nodeKey,
		peerManager: peerManager,
		router:      router,

		pexReactor:   pexReactor,
		pexReactorV2: pexReactorV2,
	}
	node.BaseService = *service.NewBaseService(logger, "SeedNode", node)

	for _, option := range options {
		option(node)
	}

	return node, nil
}

// Option sets a parameter for the node.
type Option func(*Node)

// Temporary interface for switching to fast sync, we should get rid of v0.
// See: https://github.com/tendermint/tendermint/issues/4595
type fastSyncReactor interface {
	SwitchToFastSync(sm.State) error
}

// CustomReactors allows you to add custom reactors (name -> p2p.Reactor) to
// the node's Switch.
//
// WARNING: using any name from the below list of the existing reactors will
// result in replacing it with the custom one.
//
//  - MEMPOOL
//  - BLOCKCHAIN
//  - CONSENSUS
//  - EVIDENCE
//  - PEX
//  - STATESYNC
func CustomReactors(reactors map[string]p2p.Reactor) Option {
	return func(n *Node) {
		for name, reactor := range reactors {
			if existingReactor := n.sw.Reactor(name); existingReactor != nil {
				n.sw.Logger.Info("Replacing existing reactor with a custom one",
					"name", name, "existing", existingReactor, "custom", reactor)
				n.sw.RemoveReactor(name, existingReactor)
			}
			n.sw.AddReactor(name, reactor)
		}
	}
}

// StateProvider overrides the state provider used by state sync to retrieve trusted app hashes and
// build a State object for bootstrapping the node.
// WARNING: this interface is considered unstable and subject to change.
func StateProvider(stateProvider statesync.StateProvider) Option {
	return func(n *Node) {
		n.stateSyncProvider = stateProvider
	}
}

// OnStart starts the Node. It implements service.Service.
func (n *Node) OnStart() error {
	now := tmtime.Now()
	genTime := n.genesisDoc.GenesisTime
	if genTime.After(now) {
		n.Logger.Info("Genesis time is in the future. Sleeping until then...", "genTime", genTime)
		time.Sleep(genTime.Sub(now))
	}

	// Start the RPC server before the P2P server
	// so we can eg. receive txs for the first block
	if n.config.RPC.ListenAddress != "" && n.config.Mode != cfg.ModeSeed {
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
	addr, err := p2p.NewNetAddressString(p2p.IDAddressString(n.nodeKey.ID, n.config.P2P.ListenAddress))
	if err != nil {
		return err
	}
	if err := n.transport.Listen(addr.Endpoint()); err != nil {
		return err
	}

	n.isListening = true

	n.Logger.Info("p2p service", "legacy_enabled", !n.config.P2P.DisableLegacy)

	if n.config.P2P.DisableLegacy {
		err = n.router.Start()
	} else {
		// Add private IDs to addrbook to block those peers being added
		n.addrBook.AddPrivateIDs(strings.SplitAndTrimEmpty(n.config.P2P.PrivatePeerIDs, ",", " "))
		err = n.sw.Start()
	}
	if err != nil {
		return err
	}

	if n.config.Mode != cfg.ModeSeed {
		if n.config.FastSync.Version == cfg.BlockchainV0 {
			// Start the real blockchain reactor separately since the switch uses the shim.
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

	if n.config.P2P.DisableLegacy && n.pexReactorV2 != nil {
		if err := n.pexReactorV2.Start(); err != nil {
			return err
		}
	} else {
		// Always connect to persistent peers
		err = n.sw.DialPeersAsync(strings.SplitAndTrimEmpty(n.config.P2P.PersistentPeers, ",", " "))
		if err != nil {
			return fmt.Errorf("could not dial peers from persistent-peers field: %w", err)
		}

	}

	// Run state sync
	if n.stateSync {
		bcR, ok := n.bcReactor.(fastSyncReactor)
		if !ok {
			return fmt.Errorf("this blockchain reactor does not support switching from state sync")
		}
		err := startStateSync(n.stateSyncReactor, bcR, n.consensusReactor, n.stateSyncProvider,
			n.config.StateSync, n.config.FastSyncMode, n.stateStore, n.blockStore, n.stateSyncGenesis)
		if err != nil {
			return fmt.Errorf("failed to start state sync: %w", err)
		}
	}

	return nil
}

// OnStop stops the Node. It implements service.Service.
func (n *Node) OnStop() {

	n.Logger.Info("Stopping Node")

	// first stop the non-reactor services
	if err := n.eventBus.Stop(); err != nil {
		n.Logger.Error("Error closing eventBus", "err", err)
	}
	if err := n.indexerService.Stop(); err != nil {
		n.Logger.Error("Error closing indexerService", "err", err)
	}

	if n.config.Mode != cfg.ModeSeed {
		// now stop the reactors
		if n.config.FastSync.Version == cfg.BlockchainV0 {
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

	if n.config.P2P.DisableLegacy && n.pexReactorV2 != nil {
		if err := n.pexReactorV2.Stop(); err != nil {
			n.Logger.Error("failed to stop the PEX v2 reactor", "err", err)
		}
	}

	if n.config.P2P.DisableLegacy {
		if err := n.router.Stop(); err != nil {
			n.Logger.Error("failed to stop router", "err", err)
		}
	} else {
		if err := n.sw.Stop(); err != nil {
			n.Logger.Error("failed to stop switch", "err", err)
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

// ConfigureRPC makes sure RPC has all the objects it needs to operate.
func (n *Node) ConfigureRPC() (*rpccore.Environment, error) {
	rpcCoreEnv := rpccore.Environment{
		ProxyAppQuery:   n.proxyApp.Query(),
		ProxyAppMempool: n.proxyApp.Mempool(),

		StateStore:     n.stateStore,
		BlockStore:     n.blockStore,
		EvidencePool:   n.evidencePool,
		ConsensusState: n.consensusState,
		P2PPeers:       n.sw,
		P2PTransport:   n,

		GenDoc:           n.genesisDoc,
		TxIndexer:        n.txIndexer,
		BlockIndexer:     n.blockIndexer,
		ConsensusReactor: n.consensusReactor,
		EventBus:         n.eventBus,
		Mempool:          n.mempool,

		Logger: n.Logger.With("module", "rpc"),

		Config: *n.config.RPC,
	}
	if n.config.Mode == cfg.ModeValidator {
		pubKey, err := n.privValidator.GetPubKey(context.TODO())
		if pubKey == nil || err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		rpcCoreEnv.PubKey = pubKey
	}
	if err := rpcCoreEnv.InitGenesisChunks(); err != nil {
		return nil, err
	}

	return &rpcCoreEnv, nil
}

func (n *Node) startRPC() ([]net.Listener, error) {
	env, err := n.ConfigureRPC()
	if err != nil {
		return nil, err
	}

	listenAddrs := strings.SplitAndTrimEmpty(n.config.RPC.ListenAddress, ",", " ")
	routes := env.GetRoutes()

	if n.config.RPC.Unsafe {
		env.AddUnsafe(routes)
	}

	config := rpcserver.DefaultConfig()
	config.MaxBodyBytes = n.config.RPC.MaxBodyBytes
	config.MaxHeaderBytes = n.config.RPC.MaxHeaderBytes
	config.MaxOpenConnections = n.config.RPC.MaxOpenConnections
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	if config.WriteTimeout <= n.config.RPC.TimeoutBroadcastTxCommit {
		config.WriteTimeout = n.config.RPC.TimeoutBroadcastTxCommit + 1*time.Second
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
			rpcserver.ReadLimit(config.MaxBodyBytes),
		)
		wm.SetLogger(wmLogger)
		mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(mux, routes, rpcLogger)
		listener, err := rpcserver.Listen(
			listenAddr,
			config,
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
					config,
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
					config,
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
		config := rpcserver.DefaultConfig()
		config.MaxBodyBytes = n.config.RPC.MaxBodyBytes
		config.MaxHeaderBytes = n.config.RPC.MaxHeaderBytes
		// NOTE: GRPCMaxOpenConnections is used, not MaxOpenConnections
		config.MaxOpenConnections = n.config.RPC.GRPCMaxOpenConnections
		// If necessary adjust global WriteTimeout to ensure it's greater than
		// TimeoutBroadcastTxCommit.
		// See https://github.com/tendermint/tendermint/issues/3435
		if config.WriteTimeout <= n.config.RPC.TimeoutBroadcastTxCommit {
			config.WriteTimeout = n.config.RPC.TimeoutBroadcastTxCommit + 1*time.Second
		}
		listener, err := rpcserver.Listen(grpcListenAddr, config)
		if err != nil {
			return nil, err
		}
		go func() {
			if err := grpccore.StartGRPCServer(env, listener); err != nil {
				n.Logger.Error("Error starting gRPC server", "err", err)
			}
		}()
		listeners = append(listeners, listener)

	}

	return listeners, nil

}

// startPrometheusServer starts a Prometheus HTTP server, listening for metrics
// collectors on addr.
func (n *Node) startPrometheusServer(addr string) *http.Server {
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

// Switch returns the Node's Switch.
func (n *Node) Switch() *p2p.Switch {
	return n.sw
}

// BlockStore returns the Node's BlockStore.
func (n *Node) BlockStore() *store.BlockStore {
	return n.blockStore
}

// ConsensusState returns the Node's ConsensusState.
func (n *Node) ConsensusState() *cs.State {
	return n.consensusState
}

// ConsensusReactor returns the Node's ConsensusReactor.
func (n *Node) ConsensusReactor() *cs.Reactor {
	return n.consensusReactor
}

// MempoolReactor returns the Node's mempool reactor.
func (n *Node) MempoolReactor() *mempl.Reactor {
	return n.mempoolReactor
}

// Mempool returns the Node's mempool.
func (n *Node) Mempool() mempl.Mempool {
	return n.mempool
}

// PEXReactor returns the Node's PEXReactor. It returns nil if PEX is disabled.
func (n *Node) PEXReactor() *pex.Reactor {
	return n.pexReactor
}

// EvidencePool returns the Node's EvidencePool.
func (n *Node) EvidencePool() *evidence.Pool {
	return n.evidencePool
}

// EventBus returns the Node's EventBus.
func (n *Node) EventBus() *types.EventBus {
	return n.eventBus
}

// PrivValidator returns the Node's PrivValidator.
// XXX: for convenience only!
func (n *Node) PrivValidator() types.PrivValidator {
	return n.privValidator
}

// GenesisDoc returns the Node's GenesisDoc.
func (n *Node) GenesisDoc() *types.GenesisDoc {
	return n.genesisDoc
}

// ProxyApp returns the Node's AppConns, representing its connections to the ABCI application.
func (n *Node) ProxyApp() proxy.AppConns {
	return n.proxyApp
}

// Config returns the Node's config.
func (n *Node) Config() *cfg.Config {
	return n.config
}

// TxIndexer returns the Node's TxIndexer.
func (n *Node) TxIndexer() indexer.TxIndexer {
	return n.txIndexer
}

//------------------------------------------------------------------------------

func (n *Node) Listeners() []string {
	return []string{
		fmt.Sprintf("Listener(@%v)", n.config.P2P.ExternalAddress),
	}
}

func (n *Node) IsListening() bool {
	return n.isListening
}

// NodeInfo returns the Node's Info from the Switch.
func (n *Node) NodeInfo() p2p.NodeInfo {
	return n.nodeInfo
}

// startStateSync starts an asynchronous state sync process, then switches to fast sync mode.
func startStateSync(ssR *statesync.Reactor, bcR fastSyncReactor, conR *cs.Reactor,
	stateProvider statesync.StateProvider, config *cfg.StateSyncConfig, fastSync bool,
	stateStore sm.Store, blockStore *store.BlockStore, state sm.State) error {
	ssR.Logger.Info("Starting state sync")

	if stateProvider == nil {
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stateProvider, err = statesync.NewLightClientStateProvider(
			ctx,
			state.ChainID, state.Version, state.InitialHeight,
			config.RPCServers, light.TrustOptions{
				Period: config.TrustPeriod,
				Height: config.TrustHeight,
				Hash:   config.TrustHashBytes(),
			}, ssR.Logger.With("module", "light"))
		if err != nil {
			return fmt.Errorf("failed to set up light client state provider: %w", err)
		}
	}

	go func() {
		err := ssR.Sync(stateProvider, config.DiscoveryTime)
		if err != nil {
			ssR.Logger.Error("State sync failed", "err", err)
			return
		}

		state, err := stateStore.Load()
		if err != nil {
			ssR.Logger.Error("failed to load state", "err", err)
		}

		if fastSync {
			// FIXME Very ugly to have these metrics bleed through here.
			conR.Metrics.StateSyncing.Set(0)
			conR.Metrics.FastSyncing.Set(1)
			err = bcR.SwitchToFastSync(state)
			if err != nil {
				ssR.Logger.Error("Failed to switch to fast sync", "err", err)
				return
			}
		} else {
			conR.SwitchToConsensus(state, true)
		}
	}()
	return nil
}

// DBContext specifies config information for loading a new DB.
type DBContext struct {
	ID     string
	Config *cfg.Config
}

// DBProvider takes a DBContext and returns an instantiated DB.
type DBProvider func(*DBContext) (dbm.DB, error)

// DefaultDBProvider returns a database using the DBBackend and DBDir
// specified in the ctx.Config.
func DefaultDBProvider(ctx *DBContext) (dbm.DB, error) {
	dbType := dbm.BackendType(ctx.Config.DBBackend)
	return dbm.NewDB(ctx.ID, dbType, ctx.Config.DBDir())
}

// GenesisDocProvider returns a GenesisDoc.
// It allows the GenesisDoc to be pulled from sources other than the
// filesystem, for instance from a distributed key-value store cluster.
type GenesisDocProvider func() (*types.GenesisDoc, error)

// DefaultGenesisDocProviderFunc returns a GenesisDocProvider that loads
// the GenesisDoc from the config.GenesisFile() on the filesystem.
func DefaultGenesisDocProviderFunc(config *cfg.Config) GenesisDocProvider {
	return func() (*types.GenesisDoc, error) {
		return types.GenesisDocFromFile(config.GenesisFile())
	}
}

// Provider takes a config and a logger and returns a ready to go Node.
type Provider func(*cfg.Config, log.Logger) (*Node, error)

// MetricsProvider returns a consensus, p2p and mempool Metrics.
type MetricsProvider func(chainID string) (*cs.Metrics, *p2p.Metrics, *mempl.Metrics, *sm.Metrics)

// DefaultMetricsProvider returns Metrics build using Prometheus client library
// if Prometheus is enabled. Otherwise, it returns no-op Metrics.
func DefaultMetricsProvider(config *cfg.InstrumentationConfig) MetricsProvider {
	return func(chainID string) (*cs.Metrics, *p2p.Metrics, *mempl.Metrics, *sm.Metrics) {
		if config.Prometheus {
			return cs.PrometheusMetrics(config.Namespace, "chain_id", chainID),
				p2p.PrometheusMetrics(config.Namespace, "chain_id", chainID),
				mempl.PrometheusMetrics(config.Namespace, "chain_id", chainID),
				sm.PrometheusMetrics(config.Namespace, "chain_id", chainID)
		}
		return cs.NopMetrics(), p2p.NopMetrics(), mempl.NopMetrics(), sm.NopMetrics()
	}
}

//------------------------------------------------------------------------------

var (
	genesisDocKey = []byte("genesisDoc")
)

// LoadStateFromDBOrGenesisDocProvider attempts to load the state from the
// database, or creates one using the given genesisDocProvider. On success this also
// returns the genesis doc loaded through the given provider.
func LoadStateFromDBOrGenesisDocProvider(
	stateDB dbm.DB,
	genesisDocProvider GenesisDocProvider,
) (sm.State, *types.GenesisDoc, error) {
	// Get genesis doc
	genDoc, err := loadGenesisDoc(stateDB)
	if err != nil {
		genDoc, err = genesisDocProvider()
		if err != nil {
			return sm.State{}, nil, err
		}

		err = genDoc.ValidateAndComplete()
		if err != nil {
			return sm.State{}, nil, fmt.Errorf("error in genesis doc: %w", err)
		}
		// save genesis doc to prevent a certain class of user errors (e.g. when it
		// was changed, accidentally or not). Also good for audit trail.
		if err := saveGenesisDoc(stateDB, genDoc); err != nil {
			return sm.State{}, nil, err
		}
	}
	stateStore := sm.NewStore(stateDB)
	state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
	if err != nil {
		return sm.State{}, nil, err
	}
	return state, genDoc, nil
}

// panics if failed to unmarshal bytes
func loadGenesisDoc(db dbm.DB) (*types.GenesisDoc, error) {
	b, err := db.Get(genesisDocKey)
	if err != nil {
		panic(err)
	}
	if len(b) == 0 {
		return nil, errors.New("genesis doc not found")
	}
	var genDoc *types.GenesisDoc
	err = tmjson.Unmarshal(b, &genDoc)
	if err != nil {
		panic(fmt.Sprintf("Failed to load genesis doc due to unmarshaling error: %v (bytes: %X)", err, b))
	}
	return genDoc, nil
}

// panics if failed to marshal the given genesis document
func saveGenesisDoc(db dbm.DB, genDoc *types.GenesisDoc) error {
	b, err := tmjson.Marshal(genDoc)
	if err != nil {
		return fmt.Errorf("failed to save genesis doc due to marshaling error: %w", err)
	}
	if err := db.SetSync(genesisDocKey, b); err != nil {
		return err
	}

	return nil
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
	config *cfg.Config,
	chainID string,
	logger log.Logger,
) (types.PrivValidator, error) {
	pvsc, err := tmgrpc.DialRemoteSigner(config, chainID, logger)
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

func getRouterConfig(conf *cfg.Config, proxyApp proxy.AppConns) p2p.RouterOptions {
	opts := p2p.RouterOptions{
		QueueType: conf.P2P.QueueType,
	}

	if conf.P2P.MaxNumInboundPeers > 0 {
		opts.MaxIncomingConnectionAttempts = conf.P2P.MaxIncomingConnectionAttempts
	}

	if conf.FilterPeers && proxyApp != nil {
		opts.FilterPeerByID = func(ctx context.Context, id p2p.NodeID) error {
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
