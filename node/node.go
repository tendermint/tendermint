package node

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	bc "github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/evidence"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/pex"
	"github.com/tendermint/tendermint/p2p/trust"
	"github.com/tendermint/tendermint/proxy"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	grpccore "github.com/tendermint/tendermint/rpc/grpc"
	rpc "github.com/tendermint/tendermint/rpc/lib"
	rpcserver "github.com/tendermint/tendermint/rpc/lib/server"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/state/txindex/kv"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
	priv_val "github.com/tendermint/tendermint/types/priv_validator"
	"github.com/tendermint/tendermint/version"

	_ "net/http/pprof"
)

//------------------------------------------------------------------------------

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
	dbType := dbm.DBBackendType(ctx.Config.DBBackend)
	return dbm.NewDB(ctx.ID, dbType, ctx.Config.DBDir()), nil
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

// NodeProvider takes a config and a logger and returns a ready to go Node.
type NodeProvider func(*cfg.Config, log.Logger) (*Node, error)

// DefaultNewNode returns a Tendermint node with default settings for the
// PrivValidator, ClientCreator, GenesisDoc, and DBProvider.
// It implements NodeProvider.
func DefaultNewNode(config *cfg.Config, logger log.Logger) (*Node, error) {
	return NewNode(config,
		types.LoadOrGenPrivValidatorFS(config.PrivValidatorFile()),
		proxy.DefaultClientCreator(config.ProxyApp, config.ABCI, config.DBDir()),
		DefaultGenesisDocProviderFunc(config),
		DefaultDBProvider,
		logger,
	)
}

//------------------------------------------------------------------------------

// Node is the highest level interface to a full Tendermint node.
// It includes all configuration information and running services.
type Node struct {
	cmn.BaseService

	// config
	config        *cfg.Config
	genesisDoc    *types.GenesisDoc   // initial validator set
	privValidator types.PrivValidator // local node's validator key

	// network
	sw               *p2p.Switch             // p2p connections
	addrBook         pex.AddrBook            // known peers
	trustMetricStore *trust.TrustMetricStore // trust metrics for all peers

	// services
	eventBus         *types.EventBus // pub/sub for services
	stateDB          dbm.DB
	blockStore       *bc.BlockStore         // store the blockchain to disk
	bcReactor        *bc.BlockchainReactor  // for fast-syncing
	mempoolReactor   *mempl.MempoolReactor  // for gossipping transactions
	consensusState   *cs.ConsensusState     // latest consensus state
	consensusReactor *cs.ConsensusReactor   // for participating in the consensus
	evidencePool     *evidence.EvidencePool // tracking evidence
	proxyApp         proxy.AppConns         // connection to the application
	rpcListeners     []net.Listener         // rpc servers
	txIndexer        txindex.TxIndexer
	indexerService   *txindex.IndexerService
}

// NewNode returns a new, ready to go, Tendermint Node.
func NewNode(config *cfg.Config,
	privValidator types.PrivValidator,
	clientCreator proxy.ClientCreator,
	genesisDocProvider GenesisDocProvider,
	dbProvider DBProvider,
	logger log.Logger) (*Node, error) {

	// Get BlockStore
	blockStoreDB, err := dbProvider(&DBContext{"blockstore", config})
	if err != nil {
		return nil, err
	}
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB, err := dbProvider(&DBContext{"state", config})
	if err != nil {
		return nil, err
	}

	// Get genesis doc
	// TODO: move to state package?
	genDoc, err := loadGenesisDoc(stateDB)
	if err != nil {
		genDoc, err = genesisDocProvider()
		if err != nil {
			return nil, err
		}
		// save genesis doc to prevent a certain class of user errors (e.g. when it
		// was changed, accidentally or not). Also good for audit trail.
		saveGenesisDoc(stateDB, genDoc)
	}

	state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
	if err != nil {
		return nil, err
	}

	// Create the proxyApp, which manages connections (consensus, mempool, query)
	// and sync tendermint and the app by performing a handshake
	// and replaying any necessary blocks
	consensusLogger := logger.With("module", "consensus")
	handshaker := cs.NewHandshaker(stateDB, state, blockStore, genDoc.AppState())
	handshaker.SetLogger(consensusLogger)
	proxyApp := proxy.NewAppConns(clientCreator, handshaker)
	proxyApp.SetLogger(logger.With("module", "proxy"))
	if err := proxyApp.Start(); err != nil {
		return nil, fmt.Errorf("Error starting proxy app connections: %v", err)
	}

	// reload the state (it may have been updated by the handshake)
	state = sm.LoadState(stateDB)

	// If an address is provided, listen on the socket for a
	// connection from an external signing process.
	if config.PrivValidatorListenAddr != "" {
		var (
			// TODO: persist this key so external signer
			// can actually authenticate us
			privKey = crypto.GenPrivKeyEd25519()
			pvsc    = priv_val.NewSocketClient(
				logger.With("module", "priv_val"),
				config.PrivValidatorListenAddr,
				privKey,
			)
		)

		if err := pvsc.Start(); err != nil {
			return nil, fmt.Errorf("Error starting private validator client: %v", err)
		}

		privValidator = pvsc
	}

	// Decide whether to fast-sync or not
	// We don't fast-sync when the only validator is us.
	fastSync := config.FastSync
	if state.Validators.Size() == 1 {
		addr, _ := state.Validators.GetByIndex(0)
		if bytes.Equal(privValidator.GetAddress(), addr) {
			fastSync = false
		}
	}

	// Log whether this node is a validator or an observer
	if state.Validators.HasAddress(privValidator.GetAddress()) {
		consensusLogger.Info("This node is a validator", "addr", privValidator.GetAddress(), "pubKey", privValidator.GetPubKey())
	} else {
		consensusLogger.Info("This node is not a validator", "addr", privValidator.GetAddress(), "pubKey", privValidator.GetPubKey())
	}

	// Make MempoolReactor
	mempoolLogger := logger.With("module", "mempool")
	mempool := mempl.NewMempool(config.Mempool, proxyApp.Mempool(), state.LastBlockHeight)
	mempool.InitWAL() // no need to have the mempool wal during tests
	mempool.SetLogger(mempoolLogger)
	mempoolReactor := mempl.NewMempoolReactor(config.Mempool, mempool)
	mempoolReactor.SetLogger(mempoolLogger)

	if config.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	// Make Evidence Reactor
	evidenceDB, err := dbProvider(&DBContext{"evidence", config})
	if err != nil {
		return nil, err
	}
	evidenceLogger := logger.With("module", "evidence")
	evidenceStore := evidence.NewEvidenceStore(evidenceDB)
	evidencePool := evidence.NewEvidencePool(stateDB, evidenceStore)
	evidencePool.SetLogger(evidenceLogger)
	evidenceReactor := evidence.NewEvidenceReactor(evidencePool)
	evidenceReactor.SetLogger(evidenceLogger)

	blockExecLogger := logger.With("module", "state")
	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := sm.NewBlockExecutor(stateDB, blockExecLogger, proxyApp.Consensus(), mempool, evidencePool)

	// Make BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	// Make ConsensusReactor
	consensusState := cs.NewConsensusState(config.Consensus, state.Copy(),
		blockExec, blockStore, mempool, evidencePool)
	consensusState.SetLogger(consensusLogger)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := cs.NewConsensusReactor(consensusState, fastSync)
	consensusReactor.SetLogger(consensusLogger)

	p2pLogger := logger.With("module", "p2p")

	sw := p2p.NewSwitch(config.P2P)
	sw.SetLogger(p2pLogger)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)
	sw.AddReactor("EVIDENCE", evidenceReactor)

	// Optionally, start the pex reactor
	var addrBook pex.AddrBook
	var trustMetricStore *trust.TrustMetricStore
	if config.P2P.PexReactor {
		addrBook = pex.NewAddrBook(config.P2P.AddrBookFile(), config.P2P.AddrBookStrict)
		addrBook.SetLogger(p2pLogger.With("book", config.P2P.AddrBookFile()))

		// Get the trust metric history data
		trustHistoryDB, err := dbProvider(&DBContext{"trusthistory", config})
		if err != nil {
			return nil, err
		}
		trustMetricStore = trust.NewTrustMetricStore(trustHistoryDB, trust.DefaultConfig())
		trustMetricStore.SetLogger(p2pLogger)

		pexReactor := pex.NewPEXReactor(addrBook,
			&pex.PEXReactorConfig{
				Seeds:          cmn.SplitAndTrim(config.P2P.Seeds, ",", " "),
				SeedMode:       config.P2P.SeedMode,
				PrivatePeerIDs: cmn.SplitAndTrim(config.P2P.PrivatePeerIDs, ",", " ")})
		pexReactor.SetLogger(p2pLogger)
		sw.AddReactor("PEX", pexReactor)
	}

	sw.SetAddrBook(addrBook)

	// Filter peers by addr or pubkey with an ABCI query.
	// If the query return code is OK, add peer.
	// XXX: Query format subject to change
	if config.FilterPeers {
		// NOTE: addr is ip:port
		sw.SetAddrFilter(func(addr net.Addr) error {
			resQuery, err := proxyApp.Query().QuerySync(abci.RequestQuery{Path: cmn.Fmt("/p2p/filter/addr/%s", addr.String())})
			if err != nil {
				return err
			}
			if resQuery.IsErr() {
				return fmt.Errorf("Error querying abci app: %v", resQuery)
			}
			return nil
		})
		sw.SetIDFilter(func(id p2p.ID) error {
			resQuery, err := proxyApp.Query().QuerySync(abci.RequestQuery{Path: cmn.Fmt("/p2p/filter/pubkey/%s", id)})
			if err != nil {
				return err
			}
			if resQuery.IsErr() {
				return fmt.Errorf("Error querying abci app: %v", resQuery)
			}
			return nil
		})
	}

	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.With("module", "events"))

	// services which will be publishing and/or subscribing for messages (events)
	// consensusReactor will set it on consensusState and blockExecutor
	consensusReactor.SetEventBus(eventBus)

	// Transaction indexing
	var txIndexer txindex.TxIndexer
	switch config.TxIndex.Indexer {
	case "kv":
		store, err := dbProvider(&DBContext{"tx_index", config})
		if err != nil {
			return nil, err
		}
		if config.TxIndex.IndexTags != "" {
			txIndexer = kv.NewTxIndex(store, kv.IndexTags(cmn.SplitAndTrim(config.TxIndex.IndexTags, ",", " ")))
		} else if config.TxIndex.IndexAllTags {
			txIndexer = kv.NewTxIndex(store, kv.IndexAllTags())
		} else {
			txIndexer = kv.NewTxIndex(store)
		}
	default:
		txIndexer = &null.TxIndex{}
	}

	indexerService := txindex.NewIndexerService(txIndexer, eventBus)

	// run the profile server
	profileHost := config.ProfListenAddress
	if profileHost != "" {
		go func() {
			logger.Error("Profile server", "err", http.ListenAndServe(profileHost, nil))
		}()
	}

	node := &Node{
		config:        config,
		genesisDoc:    genDoc,
		privValidator: privValidator,

		sw:               sw,
		addrBook:         addrBook,
		trustMetricStore: trustMetricStore,

		stateDB:          stateDB,
		blockStore:       blockStore,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		evidencePool:     evidencePool,
		proxyApp:         proxyApp,
		txIndexer:        txIndexer,
		indexerService:   indexerService,
		eventBus:         eventBus,
	}
	node.BaseService = *cmn.NewBaseService(logger, "Node", node)
	return node, nil
}

// OnStart starts the Node. It implements cmn.Service.
func (n *Node) OnStart() error {
	err := n.eventBus.Start()
	if err != nil {
		return err
	}

	// Run the RPC server first
	// so we can eg. receive txs for the first block
	if n.config.RPC.ListenAddress != "" {
		listeners, err := n.startRPC()
		if err != nil {
			return err
		}
		n.rpcListeners = listeners
	}

	// Create & add listener
	protocol, address := cmn.ProtocolAndAddress(n.config.P2P.ListenAddress)
	l := p2p.NewDefaultListener(protocol, address, n.config.P2P.SkipUPNP, n.Logger.With("module", "p2p"))
	n.sw.AddListener(l)

	// Generate node PrivKey
	// TODO: pass in like privValidator
	nodeKey, err := p2p.LoadOrGenNodeKey(n.config.NodeKeyFile())
	if err != nil {
		return err
	}
	n.Logger.Info("P2P Node ID", "ID", nodeKey.ID(), "file", n.config.NodeKeyFile())

	nodeInfo := n.makeNodeInfo(nodeKey.PubKey())
	n.sw.SetNodeInfo(nodeInfo)
	n.sw.SetNodeKey(nodeKey)

	// Add ourselves to addrbook to prevent dialing ourselves
	n.addrBook.AddOurAddress(nodeInfo.NetAddress())

	// Start the switch
	err = n.sw.Start()
	if err != nil {
		return err
	}

	// Always connect to persistent peers
	if n.config.P2P.PersistentPeers != "" {
		err = n.sw.DialPeersAsync(n.addrBook, cmn.SplitAndTrim(n.config.P2P.PersistentPeers, ",", " "), true)
		if err != nil {
			return err
		}
	}

	// start tx indexer
	return n.indexerService.Start()
}

// OnStop stops the Node. It implements cmn.Service.
func (n *Node) OnStop() {
	n.BaseService.OnStop()

	n.Logger.Info("Stopping Node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()

	for _, l := range n.rpcListeners {
		n.Logger.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			n.Logger.Error("Error closing listener", "listener", l, "err", err)
		}
	}

	n.eventBus.Stop()
	n.indexerService.Stop()

	if pvsc, ok := n.privValidator.(*priv_val.SocketClient); ok {
		if err := pvsc.Stop(); err != nil {
			n.Logger.Error("Error stopping priv validator socket client", "err", err)
		}
	}
}

// RunForever waits for an interrupt signal and stops the node.
func (n *Node) RunForever() {
	// Sleep forever and then...
	cmn.TrapSignal(func() {
		n.Stop()
	})
}

// AddListener adds a listener to accept inbound peer connections.
// It should be called before starting the Node.
// The first listener is the primary listener (in NodeInfo)
func (n *Node) AddListener(l p2p.Listener) {
	n.sw.AddListener(l)
}

// ConfigureRPC sets all variables in rpccore so they will serve
// rpc calls from this node
func (n *Node) ConfigureRPC() {
	rpccore.SetStateDB(n.stateDB)
	rpccore.SetBlockStore(n.blockStore)
	rpccore.SetConsensusState(n.consensusState)
	rpccore.SetMempool(n.mempoolReactor.Mempool)
	rpccore.SetEvidencePool(n.evidencePool)
	rpccore.SetSwitch(n.sw)
	rpccore.SetPubKey(n.privValidator.GetPubKey())
	rpccore.SetGenesisDoc(n.genesisDoc)
	rpccore.SetAddrBook(n.addrBook)
	rpccore.SetProxyAppQuery(n.proxyApp.Query())
	rpccore.SetTxIndexer(n.txIndexer)
	rpccore.SetConsensusReactor(n.consensusReactor)
	rpccore.SetEventBus(n.eventBus)
	rpccore.SetLogger(n.Logger.With("module", "rpc"))
}

func (n *Node) startRPC() ([]net.Listener, error) {
	n.ConfigureRPC()
	listenAddrs := cmn.SplitAndTrim(n.config.RPC.ListenAddress, ",", " ")

	if n.config.RPC.Unsafe {
		rpccore.AddUnsafeRoutes()
	}

	// we may expose the rpc over both a unix and tcp socket
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		rpcLogger := n.Logger.With("module", "rpc-server")
		wm := rpcserver.NewWebsocketManager(rpccore.Routes, rpcserver.EventSubscriber(n.eventBus))
		wm.SetLogger(rpcLogger.With("protocol", "websocket"))
		mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(mux, rpccore.Routes, rpcLogger)
		listener, err := rpcserver.StartHTTPServer(listenAddr, mux, rpcLogger)
		if err != nil {
			return nil, err
		}
		listeners[i] = listener
	}

	// we expose a simplified api over grpc for convenience to app devs
	grpcListenAddr := n.config.RPC.GRPCListenAddress
	if grpcListenAddr != "" {
		listener, err := grpccore.StartGRPCServer(grpcListenAddr)
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, listener)
	}

	return listeners, nil
}

// Switch returns the Node's Switch.
func (n *Node) Switch() *p2p.Switch {
	return n.sw
}

// BlockStore returns the Node's BlockStore.
func (n *Node) BlockStore() *bc.BlockStore {
	return n.blockStore
}

// ConsensusState returns the Node's ConsensusState.
func (n *Node) ConsensusState() *cs.ConsensusState {
	return n.consensusState
}

// ConsensusReactor returns the Node's ConsensusReactor.
func (n *Node) ConsensusReactor() *cs.ConsensusReactor {
	return n.consensusReactor
}

// MempoolReactor returns the Node's MempoolReactor.
func (n *Node) MempoolReactor() *mempl.MempoolReactor {
	return n.mempoolReactor
}

// EvidencePool returns the Node's EvidencePool.
func (n *Node) EvidencePool() *evidence.EvidencePool {
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

func (n *Node) makeNodeInfo(pubKey crypto.PubKey) p2p.NodeInfo {
	txIndexerStatus := "on"
	if _, ok := n.txIndexer.(*null.TxIndex); ok {
		txIndexerStatus = "off"
	}
	nodeInfo := p2p.NodeInfo{
		PubKey:  pubKey,
		Network: n.genesisDoc.ChainID,
		Version: version.Version,
		Channels: []byte{
			bc.BlockchainChannel,
			cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			mempl.MempoolChannel,
			evidence.EvidenceChannel,
		},
		Moniker: n.config.Moniker,
		Other: []string{
			cmn.Fmt("wire_version=%v", wire.Version),
			cmn.Fmt("p2p_version=%v", p2p.Version),
			cmn.Fmt("consensus_version=%v", cs.Version),
			cmn.Fmt("rpc_version=%v/%v", rpc.Version, rpccore.Version),
			cmn.Fmt("tx_index=%v", txIndexerStatus),
		},
	}

	if n.config.P2P.PexReactor {
		nodeInfo.Channels = append(nodeInfo.Channels, pex.PexChannel)
	}

	rpcListenAddr := n.config.RPC.ListenAddress
	nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("rpc_addr=%v", rpcListenAddr))

	if !n.sw.IsListening() {
		return nodeInfo
	}

	p2pListener := n.sw.Listeners()[0]
	p2pHost := p2pListener.ExternalAddress().IP.String()
	p2pPort := p2pListener.ExternalAddress().Port
	nodeInfo.ListenAddr = cmn.Fmt("%v:%v", p2pHost, p2pPort)

	return nodeInfo
}

//------------------------------------------------------------------------------

// NodeInfo returns the Node's Info from the Switch.
func (n *Node) NodeInfo() p2p.NodeInfo {
	return n.sw.NodeInfo()
}

//------------------------------------------------------------------------------

var (
	genesisDocKey = []byte("genesisDoc")
)

// panics if failed to unmarshal bytes
func loadGenesisDoc(db dbm.DB) (*types.GenesisDoc, error) {
	bytes := db.Get(genesisDocKey)
	if len(bytes) == 0 {
		return nil, errors.New("Genesis doc not found")
	}
	var genDoc *types.GenesisDoc
	err := json.Unmarshal(bytes, &genDoc)
	if err != nil {
		cmn.PanicCrisis(fmt.Sprintf("Failed to load genesis doc due to unmarshaling error: %v (bytes: %X)", err, bytes))
	}
	return genDoc, nil
}

// panics if failed to marshal the given genesis document
func saveGenesisDoc(db dbm.DB, genDoc *types.GenesisDoc) {
	bytes, err := json.Marshal(genDoc)
	if err != nil {
		cmn.PanicCrisis(fmt.Sprintf("Failed to save genesis doc due to marshaling error: %v", err))
	}
	db.SetSync(genesisDocKey, bytes)
}
