package node

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	_ "net/http/pprof" // nolint: gosec // securely exposed on separate, optional port
	"strings"
	"time"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	bcv0 "github.com/tendermint/tendermint/internal/blockchain/v0"
	bcv2 "github.com/tendermint/tendermint/internal/blockchain/v2"
	cs "github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/mempool"
	mempoolv0 "github.com/tendermint/tendermint/internal/mempool/v0"
	mempoolv1 "github.com/tendermint/tendermint/internal/mempool/v1"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	"github.com/tendermint/tendermint/internal/statesync"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmstrings "github.com/tendermint/tendermint/libs/strings"
	protop2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
	kv "github.com/tendermint/tendermint/state/indexer/sink/kv"
	null "github.com/tendermint/tendermint/state/indexer/sink/null"
	psql "github.com/tendermint/tendermint/state/indexer/sink/psql"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

func initDBs(config *cfg.Config, dbProvider cfg.DBProvider) (blockStore *store.BlockStore, stateDB dbm.DB, err error) {
	var blockStoreDB dbm.DB
	blockStoreDB, err = dbProvider(&cfg.DBContext{ID: "blockstore", Config: config})
	if err != nil {
		return
	}
	blockStore = store.NewBlockStore(blockStoreDB)

	stateDB, err = dbProvider(&cfg.DBContext{ID: "state", Config: config})
	return
}

func createAndStartProxyAppConns(clientCreator proxy.ClientCreator, logger log.Logger) (proxy.AppConns, error) {
	proxyApp := proxy.NewAppConns(clientCreator)
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
	config *cfg.Config,
	dbProvider cfg.DBProvider,
	eventBus *types.EventBus,
	logger log.Logger,
	chainID string,
) (*indexer.Service, []indexer.EventSink, error) {

	eventSinks := []indexer.EventSink{}

	// Check duplicated sinks.
	sinks := map[string]bool{}
	for _, s := range config.TxIndex.Indexer {
		sl := strings.ToLower(s)
		if sinks[sl] {
			return nil, nil, errors.New("found duplicated sinks, please check the tx-index section in the config.toml")
		}
		sinks[sl] = true
	}

loop:
	for k := range sinks {
		switch k {
		case string(indexer.NULL):
			// when we see null in the config, the eventsinks will be reset with the nullEventSink.
			eventSinks = []indexer.EventSink{null.NewEventSink()}
			break loop
		case string(indexer.KV):
			store, err := dbProvider(&cfg.DBContext{ID: "tx_index", Config: config})
			if err != nil {
				return nil, nil, err
			}
			eventSinks = append(eventSinks, kv.NewEventSink(store))
		case string(indexer.PSQL):
			conn := config.TxIndex.PsqlConn
			if conn == "" {
				return nil, nil, errors.New("the psql connection settings cannot be empty")
			}
			es, _, err := psql.NewEventSink(conn, chainID)
			if err != nil {
				return nil, nil, err
			}
			eventSinks = append(eventSinks, es)
		default:
			return nil, nil, errors.New("unsupported event sink type")
		}
	}

	if len(eventSinks) == 0 {
		eventSinks = []indexer.EventSink{null.NewEventSink()}
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

	handshaker := cs.NewHandshaker(stateStore, state, blockStore, genDoc)
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
	case mode == cfg.ModeFull:
		consensusLogger.Info("This node is a fullnode")
	case mode == cfg.ModeValidator:
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
	config *cfg.Config,
	proxyApp proxy.AppConns,
	state sm.State,
	memplMetrics *mempool.Metrics,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	logger log.Logger,
) (*p2p.ReactorShim, service.Service, mempool.Mempool, error) {

	logger = logger.With("module", "mempool", "version", config.Mempool.Version)
	channelShims := mempoolv0.GetChannelShims(config.Mempool)
	reactorShim := p2p.NewReactorShim(logger, "MempoolShim", channelShims)

	var (
		channels    map[p2p.ChannelID]*p2p.Channel
		peerUpdates *p2p.PeerUpdates
	)

	if config.P2P.DisableLegacy {
		channels = makeChannelsFromShims(router, channelShims)
		peerUpdates = peerManager.Subscribe()
	} else {
		channels = getChannelsFromShim(reactorShim)
		peerUpdates = reactorShim.PeerUpdates
	}

	switch config.Mempool.Version {
	case cfg.MempoolV0:
		mp := mempoolv0.NewCListMempool(
			config.Mempool,
			proxyApp.Mempool(),
			state.LastBlockHeight,
			mempoolv0.WithMetrics(memplMetrics),
			mempoolv0.WithPreCheck(sm.TxPreCheck(state)),
			mempoolv0.WithPostCheck(sm.TxPostCheck(state)),
		)

		mp.SetLogger(logger)

		reactor := mempoolv0.NewReactor(
			logger,
			config.Mempool,
			peerManager,
			mp,
			channels[mempool.MempoolChannel],
			peerUpdates,
		)

		if config.Consensus.WaitForTxs() {
			mp.EnableTxsAvailable()
		}

		return reactorShim, reactor, mp, nil

	case cfg.MempoolV1:
		mp := mempoolv1.NewTxMempool(
			logger,
			config.Mempool,
			proxyApp.Mempool(),
			state.LastBlockHeight,
			mempoolv1.WithMetrics(memplMetrics),
			mempoolv1.WithPreCheck(sm.TxPreCheck(state)),
			mempoolv1.WithPostCheck(sm.TxPostCheck(state)),
		)

		reactor := mempoolv1.NewReactor(
			logger,
			config.Mempool,
			peerManager,
			mp,
			channels[mempool.MempoolChannel],
			peerUpdates,
		)

		if config.Consensus.WaitForTxs() {
			mp.EnableTxsAvailable()
		}

		return reactorShim, reactor, mp, nil

	default:
		return nil, nil, nil, fmt.Errorf("unknown mempool version: %s", config.Mempool.Version)
	}
}

func createEvidenceReactor(
	config *cfg.Config,
	dbProvider cfg.DBProvider,
	stateDB dbm.DB,
	blockStore *store.BlockStore,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	logger log.Logger,
) (*p2p.ReactorShim, *evidence.Reactor, *evidence.Pool, error) {
	evidenceDB, err := dbProvider(&cfg.DBContext{ID: "evidence", Config: config})
	if err != nil {
		return nil, nil, nil, err
	}

	logger = logger.With("module", "evidence")
	reactorShim := p2p.NewReactorShim(logger, "EvidenceShim", evidence.ChannelShims)

	evidencePool, err := evidence.NewPool(logger, evidenceDB, sm.NewStore(stateDB), blockStore)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating evidence pool: %w", err)
	}

	var (
		channels    map[p2p.ChannelID]*p2p.Channel
		peerUpdates *p2p.PeerUpdates
	)

	if config.P2P.DisableLegacy {
		channels = makeChannelsFromShims(router, evidence.ChannelShims)
		peerUpdates = peerManager.Subscribe()
	} else {
		channels = getChannelsFromShim(reactorShim)
		peerUpdates = reactorShim.PeerUpdates
	}

	evidenceReactor := evidence.NewReactor(
		logger,
		channels[evidence.EvidenceChannel],
		peerUpdates,
		evidencePool,
	)

	return reactorShim, evidenceReactor, evidencePool, nil
}

func createBlockchainReactor(
	logger log.Logger,
	config *cfg.Config,
	state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore *store.BlockStore,
	csReactor *cs.Reactor,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	fastSync bool,
) (*p2p.ReactorShim, service.Service, error) {

	logger = logger.With("module", "blockchain")

	switch config.FastSync.Version {
	case cfg.BlockchainV0:
		reactorShim := p2p.NewReactorShim(logger, "BlockchainShim", bcv0.ChannelShims)

		var (
			channels    map[p2p.ChannelID]*p2p.Channel
			peerUpdates *p2p.PeerUpdates
		)

		if config.P2P.DisableLegacy {
			channels = makeChannelsFromShims(router, bcv0.ChannelShims)
			peerUpdates = peerManager.Subscribe()
		} else {
			channels = getChannelsFromShim(reactorShim)
			peerUpdates = reactorShim.PeerUpdates
		}

		reactor, err := bcv0.NewReactor(
			logger, state.Copy(), blockExec, blockStore, csReactor,
			channels[bcv0.BlockchainChannel], peerUpdates, fastSync,
		)
		if err != nil {
			return nil, nil, err
		}

		return reactorShim, reactor, nil

	case cfg.BlockchainV2:
		reactor := bcv2.NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)
		reactor.SetLogger(logger)

		return nil, reactor, nil

	default:
		return nil, nil, fmt.Errorf("unknown fastsync version %s", config.FastSync.Version)
	}
}

func createConsensusReactor(
	config *cfg.Config,
	state sm.State,
	blockExec *sm.BlockExecutor,
	blockStore sm.BlockStore,
	mp mempool.Mempool,
	evidencePool *evidence.Pool,
	privValidator types.PrivValidator,
	csMetrics *cs.Metrics,
	waitSync bool,
	eventBus *types.EventBus,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
	logger log.Logger,
) (*p2p.ReactorShim, *cs.Reactor, *cs.State) {

	consensusState := cs.NewState(
		config.Consensus,
		state.Copy(),
		blockExec,
		blockStore,
		mp,
		evidencePool,
		cs.StateMetrics(csMetrics),
	)
	consensusState.SetLogger(logger)
	if privValidator != nil && config.Mode == cfg.ModeValidator {
		consensusState.SetPrivValidator(privValidator)
	}

	reactorShim := p2p.NewReactorShim(logger, "ConsensusShim", cs.ChannelShims)

	var (
		channels    map[p2p.ChannelID]*p2p.Channel
		peerUpdates *p2p.PeerUpdates
	)

	if config.P2P.DisableLegacy {
		channels = makeChannelsFromShims(router, cs.ChannelShims)
		peerUpdates = peerManager.Subscribe()
	} else {
		channels = getChannelsFromShim(reactorShim)
		peerUpdates = reactorShim.PeerUpdates
	}

	reactor := cs.NewReactor(
		logger,
		consensusState,
		channels[cs.StateChannel],
		channels[cs.DataChannel],
		channels[cs.VoteChannel],
		channels[cs.VoteSetBitsChannel],
		peerUpdates,
		waitSync,
		cs.ReactorMetrics(csMetrics),
	)

	// Services which will be publishing and/or subscribing for messages (events)
	// consensusReactor will set it on consensusState and blockExecutor.
	reactor.SetEventBus(eventBus)

	return reactorShim, reactor, consensusState
}

func createTransport(logger log.Logger, config *cfg.Config) *p2p.MConnTransport {
	return p2p.NewMConnTransport(
		logger, p2p.MConnConfig(config.P2P), []*p2p.ChannelDescriptor{},
		p2p.MConnTransportOptions{
			MaxAcceptedConnections: uint32(config.P2P.MaxNumInboundPeers +
				len(tmstrings.SplitAndTrimEmpty(config.P2P.UnconditionalPeerIDs, ",", " ")),
			),
		},
	)
}

func createPeerManager(
	config *cfg.Config,
	dbProvider cfg.DBProvider,
	p2pLogger log.Logger,
	nodeID p2p.NodeID,
) (*p2p.PeerManager, error) {

	var maxConns uint16

	switch {
	case config.P2P.MaxConnections > 0:
		maxConns = config.P2P.MaxConnections

	case config.P2P.MaxNumInboundPeers > 0 && config.P2P.MaxNumOutboundPeers > 0:
		x := config.P2P.MaxNumInboundPeers + config.P2P.MaxNumOutboundPeers
		if x > math.MaxUint16 {
			return nil, fmt.Errorf(
				"max inbound peers (%d) + max outbound peers (%d) exceeds maximum (%d)",
				config.P2P.MaxNumInboundPeers,
				config.P2P.MaxNumOutboundPeers,
				math.MaxUint16,
			)
		}

		maxConns = uint16(x)

	default:
		maxConns = 64
	}

	privatePeerIDs := make(map[p2p.NodeID]struct{})
	for _, id := range tmstrings.SplitAndTrimEmpty(config.P2P.PrivatePeerIDs, ",", " ") {
		privatePeerIDs[p2p.NodeID(id)] = struct{}{}
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
	for _, p := range tmstrings.SplitAndTrimEmpty(config.P2P.PersistentPeers, ",", " ") {
		address, err := p2p.ParseNodeAddress(p)
		if err != nil {
			return nil, fmt.Errorf("invalid peer address %q: %w", p, err)
		}

		peers = append(peers, address)
		options.PersistentPeers = append(options.PersistentPeers, address.NodeID)
	}

	for _, p := range tmstrings.SplitAndTrimEmpty(config.P2P.BootstrapPeers, ",", " ") {
		address, err := p2p.ParseNodeAddress(p)
		if err != nil {
			return nil, fmt.Errorf("invalid peer address %q: %w", p, err)
		}
		peers = append(peers, address)
	}

	peerDB, err := dbProvider(&cfg.DBContext{ID: "peerstore", Config: config})
	if err != nil {
		return nil, err
	}

	peerManager, err := p2p.NewPeerManager(nodeID, peerDB, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer manager: %w", err)
	}

	for _, peer := range peers {
		if _, err := peerManager.Add(peer); err != nil {
			return nil, fmt.Errorf("failed to add peer %q: %w", peer, err)
		}
	}

	return peerManager, nil
}

func createRouter(
	p2pLogger log.Logger,
	p2pMetrics *p2p.Metrics,
	nodeInfo p2p.NodeInfo,
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

func createSwitch(
	config *cfg.Config,
	transport p2p.Transport,
	p2pMetrics *p2p.Metrics,
	mempoolReactor *p2p.ReactorShim,
	bcReactor p2p.Reactor,
	stateSyncReactor *p2p.ReactorShim,
	consensusReactor *p2p.ReactorShim,
	evidenceReactor *p2p.ReactorShim,
	proxyApp proxy.AppConns,
	nodeInfo p2p.NodeInfo,
	nodeKey p2p.NodeKey,
	p2pLogger log.Logger,
) *p2p.Switch {

	var (
		connFilters = []p2p.ConnFilterFunc{}
		peerFilters = []p2p.PeerFilterFunc{}
	)

	if !config.P2P.AllowDuplicateIP {
		connFilters = append(connFilters, p2p.ConnDuplicateIPFilter)
	}

	// Filter peers by addr or pubkey with an ABCI query.
	// If the query return code is OK, add peer.
	if config.FilterPeers {
		connFilters = append(
			connFilters,
			// ABCI query for address filtering.
			func(_ p2p.ConnSet, c net.Conn, _ []net.IP) error {
				res, err := proxyApp.Query().QuerySync(context.Background(), abci.RequestQuery{
					Path: fmt.Sprintf("/p2p/filter/addr/%s", c.RemoteAddr().String()),
				})
				if err != nil {
					return err
				}
				if res.IsErr() {
					return fmt.Errorf("error querying abci app: %v", res)
				}

				return nil
			},
		)

		peerFilters = append(
			peerFilters,
			// ABCI query for ID filtering.
			func(_ p2p.IPeerSet, p p2p.Peer) error {
				res, err := proxyApp.Query().QuerySync(context.Background(), abci.RequestQuery{
					Path: fmt.Sprintf("/p2p/filter/id/%s", p.ID()),
				})
				if err != nil {
					return err
				}
				if res.IsErr() {
					return fmt.Errorf("error querying abci app: %v", res)
				}

				return nil
			},
		)
	}

	sw := p2p.NewSwitch(
		config.P2P,
		transport,
		p2p.WithMetrics(p2pMetrics),
		p2p.SwitchPeerFilters(peerFilters...),
		p2p.SwitchConnFilters(connFilters...),
	)

	sw.SetLogger(p2pLogger)
	if config.Mode != cfg.ModeSeed {
		sw.AddReactor("MEMPOOL", mempoolReactor)
		sw.AddReactor("BLOCKCHAIN", bcReactor)
		sw.AddReactor("CONSENSUS", consensusReactor)
		sw.AddReactor("EVIDENCE", evidenceReactor)
		sw.AddReactor("STATESYNC", stateSyncReactor)
	}

	sw.SetNodeInfo(nodeInfo)
	sw.SetNodeKey(nodeKey)

	p2pLogger.Info("P2P Node ID", "ID", nodeKey.ID, "file", config.NodeKeyFile())
	return sw
}

func createAddrBookAndSetOnSwitch(config *cfg.Config, sw *p2p.Switch,
	p2pLogger log.Logger, nodeKey p2p.NodeKey) (pex.AddrBook, error) {

	addrBook := pex.NewAddrBook(config.P2P.AddrBookFile(), config.P2P.AddrBookStrict)
	addrBook.SetLogger(p2pLogger.With("book", config.P2P.AddrBookFile()))

	// Add ourselves to addrbook to prevent dialing ourselves
	if config.P2P.ExternalAddress != "" {
		addr, err := p2p.NewNetAddressString(p2p.IDAddressString(nodeKey.ID, config.P2P.ExternalAddress))
		if err != nil {
			return nil, fmt.Errorf("p2p.external_address is incorrect: %w", err)
		}
		addrBook.AddOurAddress(addr)
	}
	if config.P2P.ListenAddress != "" {
		addr, err := p2p.NewNetAddressString(p2p.IDAddressString(nodeKey.ID, config.P2P.ListenAddress))
		if err != nil {
			return nil, fmt.Errorf("p2p.laddr is incorrect: %w", err)
		}
		addrBook.AddOurAddress(addr)
	}

	sw.SetAddrBook(addrBook)

	return addrBook, nil
}

func createPEXReactorAndAddToSwitch(addrBook pex.AddrBook, config *cfg.Config,
	sw *p2p.Switch, logger log.Logger) *pex.Reactor {

	reactorConfig := &pex.ReactorConfig{
		Seeds:    tmstrings.SplitAndTrimEmpty(config.P2P.Seeds, ",", " "),
		SeedMode: config.Mode == cfg.ModeSeed,
		// See consensus/reactor.go: blocksToContributeToBecomeGoodPeer 10000
		// blocks assuming 10s blocks ~ 28 hours.
		// TODO (melekes): make it dynamic based on the actual block latencies
		// from the live network.
		// https://github.com/tendermint/tendermint/issues/3523
		SeedDisconnectWaitPeriod:     28 * time.Hour,
		PersistentPeersMaxDialPeriod: config.P2P.PersistentPeersMaxDialPeriod,
	}
	// TODO persistent peers ? so we can have their DNS addrs saved
	pexReactor := pex.NewReactor(addrBook, reactorConfig)
	pexReactor.SetLogger(logger.With("module", "pex"))
	sw.AddReactor("PEX", pexReactor)
	return pexReactor
}

func createPEXReactorV2(
	config *cfg.Config,
	logger log.Logger,
	peerManager *p2p.PeerManager,
	router *p2p.Router,
) (*pex.ReactorV2, error) {

	channel, err := router.OpenChannel(pex.ChannelDescriptor(), &protop2p.PexMessage{}, 4096)
	if err != nil {
		return nil, err
	}

	peerUpdates := peerManager.Subscribe()
	return pex.NewReactorV2(logger, peerManager, channel, peerUpdates), nil
}

func makeNodeInfo(
	config *cfg.Config,
	nodeKey p2p.NodeKey,
	eventSinks []indexer.EventSink,
	genDoc *types.GenesisDoc,
	state sm.State,
) (p2p.NodeInfo, error) {
	txIndexerStatus := "off"

	if indexer.IndexingEnabled(eventSinks) {
		txIndexerStatus = "on"
	}

	var bcChannel byte
	switch config.FastSync.Version {
	case cfg.BlockchainV0:
		bcChannel = byte(bcv0.BlockchainChannel)

	case cfg.BlockchainV2:
		bcChannel = bcv2.BlockchainChannel

	default:
		return p2p.NodeInfo{}, fmt.Errorf("unknown fastsync version %s", config.FastSync.Version)
	}

	nodeInfo := p2p.NodeInfo{
		ProtocolVersion: p2p.NewProtocolVersion(
			version.P2PProtocol, // global
			state.Version.Consensus.Block,
			state.Version.Consensus.App,
		),
		NodeID:  nodeKey.ID,
		Network: genDoc.ChainID,
		Version: version.TMVersion,
		Channels: []byte{
			bcChannel,
			byte(cs.StateChannel),
			byte(cs.DataChannel),
			byte(cs.VoteChannel),
			byte(cs.VoteSetBitsChannel),
			byte(mempool.MempoolChannel),
			byte(evidence.EvidenceChannel),
			byte(statesync.SnapshotChannel),
			byte(statesync.ChunkChannel),
			byte(statesync.LightBlockChannel),
		},
		Moniker: config.Moniker,
		Other: p2p.NodeInfoOther{
			TxIndex:    txIndexerStatus,
			RPCAddress: config.RPC.ListenAddress,
		},
	}

	if config.P2P.PexReactor {
		nodeInfo.Channels = append(nodeInfo.Channels, pex.PexChannel)
	}

	lAddr := config.P2P.ExternalAddress

	if lAddr == "" {
		lAddr = config.P2P.ListenAddress
	}

	nodeInfo.ListenAddr = lAddr

	err := nodeInfo.Validate()
	return nodeInfo, err
}

func makeSeedNodeInfo(
	config *cfg.Config,
	nodeKey p2p.NodeKey,
	genDoc *types.GenesisDoc,
	state sm.State,
) (p2p.NodeInfo, error) {
	nodeInfo := p2p.NodeInfo{
		ProtocolVersion: p2p.NewProtocolVersion(
			version.P2PProtocol, // global
			state.Version.Consensus.Block,
			state.Version.Consensus.App,
		),
		NodeID:   nodeKey.ID,
		Network:  genDoc.ChainID,
		Version:  version.TMVersion,
		Channels: []byte{},
		Moniker:  config.Moniker,
		Other: p2p.NodeInfoOther{
			TxIndex:    "off",
			RPCAddress: config.RPC.ListenAddress,
		},
	}

	if config.P2P.PexReactor {
		nodeInfo.Channels = append(nodeInfo.Channels, pex.PexChannel)
	}

	lAddr := config.P2P.ExternalAddress

	if lAddr == "" {
		lAddr = config.P2P.ListenAddress
	}

	nodeInfo.ListenAddr = lAddr

	err := nodeInfo.Validate()
	return nodeInfo, err
}
