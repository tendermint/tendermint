package node

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/blocksync"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/mempool"
	mempoolv0 "github.com/tendermint/tendermint/internal/mempool/v0"
	mempoolv1 "github.com/tendermint/tendermint/internal/mempool/v1"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/conn"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink"
	"github.com/tendermint/tendermint/internal/statesync"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmstrings "github.com/tendermint/tendermint/libs/strings"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"

	_ "net/http/pprof" // nolint: gosec // securely exposed on separate, optional port
)

type closer func() error

func makeCloser(cs []closer) closer {
	return func() error {
		errs := make([]string, 0, len(cs))
		for _, cl := range cs {
			if err := cl(); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) >= 0 {
			return errors.New(strings.Join(errs, "; "))
		}
		return nil
	}
}

func combineCloseError(err error, cl closer) error {
	if err == nil {
		return cl()
	}

	clerr := cl()
	if clerr == nil {
		return err
	}

	return fmt.Errorf("error=%q closerError=%q", err.Error(), clerr.Error())
}

func initDBs(
	cfg *config.Config,
	dbProvider config.DBProvider,
) (*store.BlockStore, dbm.DB, closer, error) {

	blockStoreDB, err := dbProvider(&config.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		return nil, nil, func() error { return nil }, err
	}
	closers := []closer{}
	blockStore := store.NewBlockStore(blockStoreDB)
	closers = append(closers, blockStoreDB.Close)

	stateDB, err := dbProvider(&config.DBContext{ID: "state", Config: cfg})
	if err != nil {
		return nil, nil, makeCloser(closers), err
	}

	closers = append(closers, stateDB.Close)

	return blockStore, stateDB, makeCloser(closers), nil
}

// nolint:lll
func createAndStartProxyAppConns(clientCreator abciclient.Creator, logger log.Logger, metrics *proxy.Metrics) (proxy.AppConns, error) {
	proxyApp := proxy.NewAppConns(clientCreator, metrics)
	proxyApp.SetLogger(logger.With("module", "proxy"))
	if err := proxyApp.Start(); err != nil {
		return nil, fmt.Errorf("error starting proxy app connections: %v", err)
	}
	return proxyApp, nil
}

func createAndStartEventBus(logger log.Logger) (*types.EventBus, error) {
	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.With("module", "events"))
	if err := eventBus.Start(); err != nil {
		return nil, err
	}
	return eventBus, nil
}

func createAndStartIndexerService(
	cfg *config.Config,
	dbProvider config.DBProvider,
	eventBus *types.EventBus,
	logger log.Logger,
	chainID string,
) (*indexer.Service, []indexer.EventSink, error) {
	eventSinks, err := sink.EventSinksFromConfig(cfg, dbProvider, chainID)
	if err != nil {
		return nil, nil, err
	}

	indexerService := indexer.NewIndexerService(eventSinks, eventBus)
	indexerService.SetLogger(logger.With("module", "txindex"))

	if err := indexerService.Start(); err != nil {
		return nil, nil, err
	}

	return indexerService, eventSinks, nil
}

func doHandshake(
	stateStore sm.Store,
	state sm.State,
	blockStore sm.BlockStore,
	genDoc *types.GenesisDoc,
	eventBus types.BlockEventPublisher,
	proxyApp proxy.AppConns,
	consensusLogger log.Logger) error {

	handshaker := consensus.NewHandshaker(stateStore, state, blockStore, genDoc)
	handshaker.SetLogger(consensusLogger)
	handshaker.SetEventBus(eventBus)
	if err := handshaker.Handshake(proxyApp); err != nil {
		return fmt.Errorf("error during handshake: %v", err)
	}
	return nil
}

func logNodeStartupInfo(state sm.State, pubKey crypto.PubKey, logger, consensusLogger log.Logger, mode string) {
	// Log the version info.
	logger.Info("Version info",
		"tmVersion", version.TMVersion,
		"block", version.BlockProtocol,
		"p2p", version.P2PProtocol,
		"mode", mode,
	)

	// If the state and software differ in block version, at least log it.
	if state.Version.Consensus.Block != version.BlockProtocol {
		logger.Info("Software and state have different block protocols",
			"software", version.BlockProtocol,
			"state", state.Version.Consensus.Block,
		)
	}
	switch {
	case mode == config.ModeFull:
		consensusLogger.Info("This node is a fullnode")
	case mode == config.ModeValidator:
		addr := pubKey.Address()
		// Log whether this node is a validator or an observer
		if state.Validators.HasAddress(addr) {
			consensusLogger.Info("This node is a validator", "addr", addr, "pubKey", pubKey.Bytes())
		} else {
			consensusLogger.Info("This node is a validator (NOT in the active validator set)",
				"addr", addr, "pubKey", pubKey.Bytes())
		}
	}
}

func onlyValidatorIsUs(state sm.State, pubKey crypto.PubKey) bool {
	if state.Validators.Size() > 1 {
		return false
	}
	addr, _ := state.Validators.GetByIndex(0)
	return pubKey != nil && bytes.Equal(pubKey.Address(), addr)
}

func createMempoolReactor(
	cfg *config.Config,
	proxyApp proxy.AppConns,
	state sm.State,
	memplMetrics *mempool.Metrics,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	logger log.Logger,
) (service.Service, mempool.Mempool, error) {

	logger = logger.With("module", "mempool", "version", cfg.Mempool.Version)
	peerUpdates := peerManager.Subscribe()

	switch cfg.Mempool.Version {
	case config.MempoolV0:
		ch, err := router.OpenChannel(mempoolv0.GetChannelDescriptor(cfg.Mempool))
		if err != nil {
			return nil, nil, err
		}

		mp := mempoolv0.NewCListMempool(
			cfg.Mempool,
			proxyApp.Mempool(),
			state.LastBlockHeight,
			mempoolv0.WithMetrics(memplMetrics),
			mempoolv0.WithPreCheck(sm.TxPreCheck(state)),
			mempoolv0.WithPostCheck(sm.TxPostCheck(state)),
		)

		mp.SetLogger(logger)

		reactor := mempoolv0.NewReactor(
			logger,
			cfg.Mempool,
			peerManager,
			mp,
			ch,
			peerUpdates,
		)

		if cfg.Consensus.WaitForTxs() {
			mp.EnableTxsAvailable()
		}

		return reactor, mp, nil

	case config.MempoolV1:
		ch, err := router.OpenChannel(mempoolv1.GetChannelDescriptor(cfg.Mempool))
		if err != nil {
			return nil, nil, err
		}

		mp := mempoolv1.NewTxMempool(
			logger,
			cfg.Mempool,
			proxyApp.Mempool(),
			state.LastBlockHeight,
			mempoolv1.WithMetrics(memplMetrics),
			mempoolv1.WithPreCheck(sm.TxPreCheck(state)),
			mempoolv1.WithPostCheck(sm.TxPostCheck(state)),
		)

		reactor := mempoolv1.NewReactor(
			logger,
			cfg.Mempool,
			peerManager,
			mp,
			ch,
			peerUpdates,
		)

		if cfg.Consensus.WaitForTxs() {
			mp.EnableTxsAvailable()
		}

		return reactor, mp, nil

	default:
		return nil, nil, fmt.Errorf("unknown mempool version: %s", cfg.Mempool.Version)
	}
}

func createEvidenceReactor(
	cfg *config.Config,
	dbProvider config.DBProvider,
	stateDB dbm.DB,
	blockStore *store.BlockStore,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	logger log.Logger,
) (*evidence.Reactor, *evidence.Pool, error) {
	evidenceDB, err := dbProvider(&config.DBContext{ID: "evidence", Config: cfg})
	if err != nil {
		return nil, nil, err
	}

	logger = logger.With("module", "evidence")

	evidencePool, err := evidence.NewPool(logger, evidenceDB, sm.NewStore(stateDB), blockStore)
	if err != nil {
		return nil, nil, fmt.Errorf("creating evidence pool: %w", err)
	}

	ch, err := router.OpenChannel(evidence.GetChannelDescriptor())
	if err != nil {
		return nil, nil, fmt.Errorf("creating evidence channel: %w", err)
	}

	evidenceReactor := evidence.NewReactor(
		logger,
		ch,
		peerManager.Subscribe(),
		evidencePool,
	)

	return evidenceReactor, evidencePool, nil
}

func createBlockchainReactor(
	logger log.Logger,
	state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore *store.BlockStore,
	csReactor *consensus.Reactor,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	blockSync bool,
	metrics *consensus.Metrics,
) (service.Service, error) {

	logger = logger.With("module", "blockchain")

	ch, err := router.OpenChannel(blocksync.GetChannelDescriptor())
	if err != nil {
		return nil, err
	}

	peerUpdates := peerManager.Subscribe()

	reactor, err := blocksync.NewReactor(
		logger, state.Copy(), blockExec, blockStore, csReactor,
		ch, peerUpdates, blockSync,
		metrics,
	)
	if err != nil {
		return nil, err
	}

	return reactor, nil
}

func createConsensusReactor(
	cfg *config.Config,
	state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore sm.BlockStore,
	mp mempool.Mempool,
	evidencePool *evidence.Pool,
	privValidator types.PrivValidator,
	csMetrics *consensus.Metrics,
	waitSync bool,
	eventBus *types.EventBus,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	logger log.Logger,
) (*consensus.Reactor, *consensus.State, error) {

	consensusState := consensus.NewState(
		cfg.Consensus,
		state.Copy(),
		blockExec,
		blockStore,
		mp,
		evidencePool,
		consensus.StateMetrics(csMetrics),
	)
	consensusState.SetLogger(logger)
	if privValidator != nil && cfg.Mode == config.ModeValidator {
		consensusState.SetPrivValidator(privValidator)
	}

	csChDesc := consensus.GetChannelDescriptors()
	channels := make(map[p2p.ChannelID]*p2p.Channel, len(csChDesc))
	for idx := range csChDesc {
		chd := csChDesc[idx]
		ch, err := router.OpenChannel(chd)
		if err != nil {
			return nil, nil, err
		}

		channels[ch.ID] = ch
	}

	peerUpdates := peerManager.Subscribe()

	reactor := consensus.NewReactor(
		logger,
		consensusState,
		channels[consensus.StateChannel],
		channels[consensus.DataChannel],
		channels[consensus.VoteChannel],
		channels[consensus.VoteSetBitsChannel],
		peerUpdates,
		waitSync,
		consensus.ReactorMetrics(csMetrics),
	)

	// Services which will be publishing and/or subscribing for messages (events)
	// consensusReactor will set it on consensusState and blockExecutor.
	reactor.SetEventBus(eventBus)

	return reactor, consensusState, nil
}

func createTransport(logger log.Logger, cfg *config.Config) *p2p.MConnTransport {
	return p2p.NewMConnTransport(
		logger, conn.DefaultMConnConfig(), []*p2p.ChannelDescriptor{},
		p2p.MConnTransportOptions{
			MaxAcceptedConnections: uint32(cfg.P2P.MaxConnections),
		},
	)
}

func createPeerManager(
	cfg *config.Config,
	dbProvider config.DBProvider,
	nodeID types.NodeID,
) (*p2p.PeerManager, closer, error) {

	var maxConns uint16

	switch {
	case cfg.P2P.MaxConnections > 0:
		maxConns = cfg.P2P.MaxConnections
	default:
		maxConns = 64
	}

	privatePeerIDs := make(map[types.NodeID]struct{})
	for _, id := range tmstrings.SplitAndTrimEmpty(cfg.P2P.PrivatePeerIDs, ",", " ") {
		privatePeerIDs[types.NodeID(id)] = struct{}{}
	}

	options := p2p.PeerManagerOptions{
		MaxConnected:           maxConns,
		MaxConnectedUpgrade:    4,
		MaxPeers:               1000,
		MinRetryTime:           100 * time.Millisecond,
		MaxRetryTime:           8 * time.Hour,
		MaxRetryTimePersistent: 5 * time.Minute,
		RetryTimeJitter:        3 * time.Second,
		PrivatePeers:           privatePeerIDs,
	}

	peers := []p2p.NodeAddress{}
	for _, p := range tmstrings.SplitAndTrimEmpty(cfg.P2P.PersistentPeers, ",", " ") {
		address, err := p2p.ParseNodeAddress(p)
		if err != nil {
			return nil, func() error { return nil }, fmt.Errorf("invalid peer address %q: %w", p, err)
		}

		peers = append(peers, address)
		options.PersistentPeers = append(options.PersistentPeers, address.NodeID)
	}

	for _, p := range tmstrings.SplitAndTrimEmpty(cfg.P2P.BootstrapPeers, ",", " ") {
		address, err := p2p.ParseNodeAddress(p)
		if err != nil {
			return nil, func() error { return nil }, fmt.Errorf("invalid peer address %q: %w", p, err)
		}
		peers = append(peers, address)
	}

	peerDB, err := dbProvider(&config.DBContext{ID: "peerstore", Config: cfg})
	if err != nil {
		return nil, func() error { return nil }, err
	}

	peerManager, err := p2p.NewPeerManager(nodeID, peerDB, options)
	if err != nil {
		return nil, peerDB.Close, fmt.Errorf("failed to create peer manager: %w", err)
	}

	for _, peer := range peers {
		if _, err := peerManager.Add(peer); err != nil {
			return nil, peerDB.Close, fmt.Errorf("failed to add peer %q: %w", peer, err)
		}
	}

	return peerManager, peerDB.Close, nil
}

func createRouter(
	p2pLogger log.Logger,
	p2pMetrics *p2p.Metrics,
	nodeInfo types.NodeInfo,
	privKey crypto.PrivKey,
	peerManager *p2p.PeerManager,
	transport p2p.Transport,
	options p2p.RouterOptions,
) (*p2p.Router, error) {

	return p2p.NewRouter(
		p2pLogger,
		p2pMetrics,
		nodeInfo,
		privKey,
		peerManager,
		[]p2p.Transport{transport},
		options,
	)
}

func createPEXReactor(
	logger log.Logger,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
) (service.Service, error) {

	channel, err := router.OpenChannel(pex.ChannelDescriptor())
	if err != nil {
		return nil, err
	}

	peerUpdates := peerManager.Subscribe()
	return pex.NewReactor(logger, peerManager, channel, peerUpdates), nil
}

func makeNodeInfo(
	cfg *config.Config,
	nodeKey types.NodeKey,
	eventSinks []indexer.EventSink,
	genDoc *types.GenesisDoc,
	state sm.State,
) (types.NodeInfo, error) {
	txIndexerStatus := "off"

	if indexer.IndexingEnabled(eventSinks) {
		txIndexerStatus = "on"
	}

	bcChannel := byte(blocksync.BlockSyncChannel)

	nodeInfo := types.NodeInfo{
		ProtocolVersion: types.ProtocolVersion{
			P2P:   version.P2PProtocol, // global
			Block: state.Version.Consensus.Block,
			App:   state.Version.Consensus.App,
		},
		NodeID:  nodeKey.ID,
		Network: genDoc.ChainID,
		Version: version.TMVersion,
		Channels: []byte{
			bcChannel,
			byte(consensus.StateChannel),
			byte(consensus.DataChannel),
			byte(consensus.VoteChannel),
			byte(consensus.VoteSetBitsChannel),
			byte(mempool.MempoolChannel),
			byte(evidence.EvidenceChannel),
			byte(statesync.SnapshotChannel),
			byte(statesync.ChunkChannel),
			byte(statesync.LightBlockChannel),
			byte(statesync.ParamsChannel),
		},
		Moniker: cfg.Moniker,
		Other: types.NodeInfoOther{
			TxIndex:    txIndexerStatus,
			RPCAddress: cfg.RPC.ListenAddress,
		},
	}

	if cfg.P2P.PexReactor {
		nodeInfo.Channels = append(nodeInfo.Channels, pex.PexChannel)
	}

	lAddr := cfg.P2P.ExternalAddress

	if lAddr == "" {
		lAddr = cfg.P2P.ListenAddress
	}

	nodeInfo.ListenAddr = lAddr

	err := nodeInfo.Validate()
	return nodeInfo, err
}

func makeSeedNodeInfo(
	cfg *config.Config,
	nodeKey types.NodeKey,
	genDoc *types.GenesisDoc,
	state sm.State,
) (types.NodeInfo, error) {
	nodeInfo := types.NodeInfo{
		ProtocolVersion: types.ProtocolVersion{
			P2P:   version.P2PProtocol, // global
			Block: state.Version.Consensus.Block,
			App:   state.Version.Consensus.App,
		},
		NodeID:   nodeKey.ID,
		Network:  genDoc.ChainID,
		Version:  version.TMVersion,
		Channels: []byte{},
		Moniker:  cfg.Moniker,
		Other: types.NodeInfoOther{
			TxIndex:    "off",
			RPCAddress: cfg.RPC.ListenAddress,
		},
	}

	if cfg.P2P.PexReactor {
		nodeInfo.Channels = append(nodeInfo.Channels, pex.PexChannel)
	}

	lAddr := cfg.P2P.ExternalAddress

	if lAddr == "" {
		lAddr = cfg.P2P.ListenAddress
	}

	nodeInfo.ListenAddr = lAddr

	err := nodeInfo.Validate()
	return nodeInfo, err
}
