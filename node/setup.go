package node

import (
	"bytes"
	"context"
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
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/evidence"
	tmstrings "github.com/tendermint/tendermint/internal/libs/strings"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/conn"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/statesync"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/privval"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
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

func convertCancelCloser(cancel context.CancelFunc) closer {
	return func() error { cancel(); return nil }
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
		return nil, nil, func() error { return nil }, fmt.Errorf("unable to initialize blockstore: %w", err)
	}
	closers := []closer{}
	blockStore := store.NewBlockStore(blockStoreDB)
	closers = append(closers, blockStoreDB.Close)

	stateDB, err := dbProvider(&config.DBContext{ID: "state", Config: cfg})
	if err != nil {
		return nil, nil, makeCloser(closers), fmt.Errorf("unable to initialize statestore: %w", err)
	}

	closers = append(closers, stateDB.Close)

	return blockStore, stateDB, makeCloser(closers), nil
}

func logNodeStartupInfo(state sm.State, pubKey crypto.PubKey, logger log.Logger, mode string) {
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

	switch mode {
	case config.ModeFull:
		logger.Info("This node is a fullnode")
	case config.ModeValidator:
		addr := pubKey.Address()
		// Log whether this node is a validator or an observer
		if state.Validators.HasAddress(addr) {
			logger.Info("This node is a validator",
				"addr", addr,
				"pubKey", pubKey.Bytes(),
			)
		} else {
			logger.Info("This node is a validator (NOT in the active validator set)",
				"addr", addr,
				"pubKey", pubKey.Bytes(),
			)
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
	logger log.Logger,
	cfg *config.Config,
	appClient abciclient.Client,
	store sm.Store,
	memplMetrics *mempool.Metrics,
	peerEvents p2p.PeerEventSubscriber,
	chCreator p2p.ChannelCreator,
) (service.Service, mempool.Mempool) {
	logger = logger.With("module", "mempool")

	mp := mempool.NewTxMempool(
		logger,
		cfg.Mempool,
		appClient,
		mempool.WithMetrics(memplMetrics),
		mempool.WithPreCheck(sm.TxPreCheckFromStore(store)),
		mempool.WithPostCheck(sm.TxPostCheckFromStore(store)),
	)

	reactor := mempool.NewReactor(
		logger,
		cfg.Mempool,
		mp,
		chCreator,
		peerEvents,
	)

	if cfg.Consensus.WaitForTxs() {
		mp.EnableTxsAvailable()
	}

	return reactor, mp
}

func createEvidenceReactor(
	logger log.Logger,
	cfg *config.Config,
	dbProvider config.DBProvider,
	store sm.Store,
	blockStore *store.BlockStore,
	peerEvents p2p.PeerEventSubscriber,
	chCreator p2p.ChannelCreator,
	metrics *evidence.Metrics,
	eventBus *eventbus.EventBus,
) (*evidence.Reactor, *evidence.Pool, closer, error) {
	evidenceDB, err := dbProvider(&config.DBContext{ID: "evidence", Config: cfg})
	if err != nil {
		return nil, nil, func() error { return nil }, fmt.Errorf("unable to initialize evidence db: %w", err)
	}

	logger = logger.With("module", "evidence")

	evidencePool := evidence.NewPool(logger, evidenceDB, store, blockStore, metrics, eventBus)
	evidenceReactor := evidence.NewReactor(logger, chCreator, peerEvents, evidencePool)

	return evidenceReactor, evidencePool, evidenceDB.Close, nil
}

func createPeerManager(
	cfg *config.Config,
	dbProvider config.DBProvider,
	nodeID types.NodeID,
	metrics *p2p.Metrics,
) (*p2p.PeerManager, closer, error) {

	selfAddr, err := p2p.ParseNodeAddress(nodeID.AddressString(cfg.P2P.ExternalAddress))
	if err != nil {
		return nil, func() error { return nil }, fmt.Errorf("couldn't parse ExternalAddress %q: %w", cfg.P2P.ExternalAddress, err)
	}

	privatePeerIDs := make(map[types.NodeID]struct{})
	for _, id := range tmstrings.SplitAndTrimEmpty(cfg.P2P.PrivatePeerIDs, ",", " ") {
		privatePeerIDs[types.NodeID(id)] = struct{}{}
	}

	var maxConns uint16

	switch {
	case cfg.P2P.MaxConnections > 0:
		maxConns = cfg.P2P.MaxConnections
	default:
		maxConns = 64
	}

	var maxOutgoingConns uint16
	switch {
	case cfg.P2P.MaxOutgoingConnections > 0:
		maxOutgoingConns = cfg.P2P.MaxOutgoingConnections
	default:
		maxOutgoingConns = maxConns / 2
	}

	maxUpgradeConns := uint16(4)

	options := p2p.PeerManagerOptions{
		SelfAddress:              selfAddr,
		MaxConnected:             maxConns,
		MaxOutgoingConnections:   maxOutgoingConns,
		MaxConnectedUpgrade:      maxUpgradeConns,
		DisconnectCooldownPeriod: 2 * time.Second,
		MaxPeers:                 maxUpgradeConns + 4*maxConns,
		MinRetryTime:             250 * time.Millisecond,
		MaxRetryTime:             30 * time.Minute,
		MaxRetryTimePersistent:   5 * time.Minute,
		RetryTimeJitter:          5 * time.Second,
		PrivatePeers:             privatePeerIDs,
		Metrics:                  metrics,
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
		return nil, func() error { return nil }, fmt.Errorf("unable to initialize peer store: %w", err)
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
	logger log.Logger,
	p2pMetrics *p2p.Metrics,
	nodeInfoProducer func() *types.NodeInfo,
	nodeKey types.NodeKey,
	peerManager *p2p.PeerManager,
	cfg *config.Config,
	appClient abciclient.Client,
) (*p2p.Router, error) {

	p2pLogger := logger.With("module", "p2p")

	transportConf := conn.DefaultMConnConfig()
	transportConf.FlushThrottle = cfg.P2P.FlushThrottleTimeout
	transportConf.SendRate = cfg.P2P.SendRate
	transportConf.RecvRate = cfg.P2P.RecvRate
	transportConf.MaxPacketMsgPayloadSize = cfg.P2P.MaxPacketMsgPayloadSize
	transport := p2p.NewMConnTransport(
		p2pLogger, transportConf, []*p2p.ChannelDescriptor{},
		p2p.MConnTransportOptions{
			MaxAcceptedConnections: uint32(cfg.P2P.MaxConnections),
		},
	)

	ep, err := p2p.NewEndpoint(nodeKey.ID.AddressString(cfg.P2P.ListenAddress))
	if err != nil {
		return nil, err
	}

	return p2p.NewRouter(
		p2pLogger,
		p2pMetrics,
		nodeKey.PrivKey,
		peerManager,
		nodeInfoProducer,
		transport,
		ep,
		getRouterConfig(cfg, appClient),
	)
}

func makeNodeInfo(
	cfg *config.Config,
	nodeKey types.NodeKey,
	eventSinks []indexer.EventSink,
	genDoc *types.GenesisDoc,
	versionInfo version.Consensus,
) (types.NodeInfo, error) {

	txIndexerStatus := "off"

	if indexer.IndexingEnabled(eventSinks) {
		txIndexerStatus = "on"
	}

	nodeInfo := types.NodeInfo{
		ProtocolVersion: types.ProtocolVersion{
			P2P:   version.P2PProtocol, // global
			Block: versionInfo.Block,
			App:   versionInfo.App,
		},
		NodeID:  nodeKey.ID,
		Network: genDoc.ChainID,
		Version: version.TMVersion,
		Channels: []byte{
			byte(blocksync.BlockSyncChannel),
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

	nodeInfo.ListenAddr = cfg.P2P.ExternalAddress
	if nodeInfo.ListenAddr == "" {
		nodeInfo.ListenAddr = cfg.P2P.ListenAddress
	}

	return nodeInfo, nodeInfo.Validate()
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
		NodeID:  nodeKey.ID,
		Network: genDoc.ChainID,
		Version: version.TMVersion,
		Channels: []byte{
			pex.PexChannel,
		},
		Moniker: cfg.Moniker,
		Other: types.NodeInfoOther{
			TxIndex:    "off",
			RPCAddress: cfg.RPC.ListenAddress,
		},
	}

	nodeInfo.ListenAddr = cfg.P2P.ExternalAddress
	if nodeInfo.ListenAddr == "" {
		nodeInfo.ListenAddr = cfg.P2P.ListenAddress
	}

	return nodeInfo, nodeInfo.Validate()
}

func createAndStartPrivValidatorSocketClient(
	ctx context.Context,
	listenAddr, chainID string,
	logger log.Logger,
) (types.PrivValidator, error) {

	pve, err := privval.NewSignerListener(listenAddr, logger)
	if err != nil {
		return nil, fmt.Errorf("starting validator listener: %w", err)
	}

	pvsc, err := privval.NewSignerClient(ctx, pve, chainID)
	if err != nil {
		return nil, fmt.Errorf("starting validator client: %w", err)
	}

	// try to get a pubkey from private validate first time
	_, err = pvsc.GetPubKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}

	const (
		timeout = 100 * time.Millisecond
		maxTime = 5 * time.Second
		retries = int(maxTime / timeout)
	)
	pvscWithRetries := privval.NewRetrySignerClient(pvsc, retries, timeout)

	return pvscWithRetries, nil
}

func createAndStartPrivValidatorGRPCClient(
	ctx context.Context,
	cfg *config.Config,
	chainID string,
	logger log.Logger,
) (types.PrivValidator, error) {
	pvsc, err := tmgrpc.DialRemoteSigner(
		ctx,
		cfg.PrivValidator,
		chainID,
		logger,
		cfg.Instrumentation.Prometheus,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start private validator: %w", err)
	}

	// try to get a pubkey from private validate first time
	_, err = pvsc.GetPubKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}

	return pvsc, nil
}

func makeDefaultPrivval(conf *config.Config) (*privval.FilePV, error) {
	if conf.Mode == config.ModeValidator {
		pval, err := privval.LoadOrGenFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
		if err != nil {
			return nil, err
		}
		return pval, nil
	}

	return nil, nil
}

func createPrivval(ctx context.Context, logger log.Logger, conf *config.Config, genDoc *types.GenesisDoc, defaultPV *privval.FilePV) (types.PrivValidator, error) {
	if conf.PrivValidator.ListenAddr != "" {
		protocol, _ := tmnet.ProtocolAndAddress(conf.PrivValidator.ListenAddr)
		// FIXME: we should return un-started services and
		// then start them later.
		switch protocol {
		case "grpc":
			privValidator, err := createAndStartPrivValidatorGRPCClient(ctx, conf, genDoc.ChainID, logger)
			if err != nil {
				return nil, fmt.Errorf("error with private validator grpc client: %w", err)
			}
			return privValidator, nil
		default:
			privValidator, err := createAndStartPrivValidatorSocketClient(
				ctx,
				conf.PrivValidator.ListenAddr,
				genDoc.ChainID,
				logger,
			)
			if err != nil {
				return nil, fmt.Errorf("error with private validator socket client: %w", err)

			}
			return privValidator, nil
		}
	}

	return defaultPV, nil
}
