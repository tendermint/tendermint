package node

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"strings"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	bc "github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	p2p "github.com/tendermint/tendermint/p2p"
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
	"github.com/tendermint/tendermint/version"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	_ "net/http/pprof"
)

type Node struct {
	cmn.BaseService

	// config
	config        *cfg.Config
	genesisDoc    *types.GenesisDoc    // initial validator set
	privValidator *types.PrivValidator // local node's validator key

	// network
	privKey  crypto.PrivKeyEd25519 // local node's p2p key
	sw       *p2p.Switch           // p2p connections
	addrBook *p2p.AddrBook         // known peers

	// services
	evsw             types.EventSwitch           // pub/sub for services
	blockStore       *bc.BlockStore              // store the blockchain to disk
	bcReactor        *bc.BlockchainReactor       // for fast-syncing
	mempoolReactor   *mempl.MempoolReactor       // for gossipping transactions
	consensusState   *consensus.ConsensusState   // latest consensus state
	consensusReactor *consensus.ConsensusReactor // for participating in the consensus
	proxyApp         proxy.AppConns              // connection to the application
	rpcListeners     []net.Listener              // rpc servers
	txIndexer        txindex.TxIndexer
}

func NewNodeDefault(config *cfg.Config, logger log.Logger) *Node {
	// Get PrivValidator
	privValidator := types.LoadOrGenPrivValidator(config.PrivValidatorFile(), logger)
	return NewNode(config, privValidator,
		proxy.DefaultClientCreator(config.ProxyApp, config.ABCI, config.DBDir()), logger)
}

func NewNode(config *cfg.Config, privValidator *types.PrivValidator, clientCreator proxy.ClientCreator, logger log.Logger) *Node {
	// Get BlockStore
	blockStoreDB := dbm.NewDB("blockstore", config.DBBackend, config.DBDir())
	blockStore := bc.NewBlockStore(blockStoreDB)

	consensusLogger := logger.With("module", "consensus")
	stateLogger := logger.With("module", "state")

	// Get State
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state := sm.GetState(stateDB, config.GenesisFile())
	state.SetLogger(stateLogger)

	// Create the proxyApp, which manages connections (consensus, mempool, query)
	// and sync tendermint and the app by replaying any necessary blocks
	handshaker := consensus.NewHandshaker(state, blockStore)
	handshaker.SetLogger(consensusLogger)
	proxyApp := proxy.NewAppConns(clientCreator, handshaker)
	proxyApp.SetLogger(logger.With("module", "proxy"))
	if _, err := proxyApp.Start(); err != nil {
		cmn.Exit(cmn.Fmt("Error starting proxy app connections: %v", err))
	}

	// reload the state (it may have been updated by the handshake)
	state = sm.LoadState(stateDB)
	state.SetLogger(stateLogger)

	// Transaction indexing
	var txIndexer txindex.TxIndexer
	switch config.TxIndex {
	case "kv":
		store := dbm.NewDB("tx_index", config.DBBackend, config.DBDir())
		txIndexer = kv.NewTxIndex(store)
	default:
		txIndexer = &null.TxIndex{}
	}
	state.TxIndexer = txIndexer

	// Generate node PrivKey
	privKey := crypto.GenPrivKeyEd25519()

	// Make event switch
	eventSwitch := types.NewEventSwitch()
	eventSwitch.SetLogger(logger.With("module", "types"))
	_, err := eventSwitch.Start()
	if err != nil {
		cmn.Exit(cmn.Fmt("Failed to start switch: %v", err))
	}

	// Decide whether to fast-sync or not
	// We don't fast-sync when the only validator is us.
	fastSync := config.FastSync
	if state.Validators.Size() == 1 {
		addr, _ := state.Validators.GetByIndex(0)
		if bytes.Equal(privValidator.Address, addr) {
			fastSync = false
		}
	}

	// Log whether this node is a validator or an observer
	if state.Validators.HasAddress(privValidator.Address) {
		consensusLogger.Info("This node is a validator")
	} else {
		consensusLogger.Info("This node is not a validator")
	}

	// Make BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state.Copy(), proxyApp.Consensus(), blockStore, fastSync)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	// Make MempoolReactor
	mempoolLogger := logger.With("module", "mempool")
	mempool := mempl.NewMempool(config.Mempool, proxyApp.Mempool())
	mempool.SetLogger(mempoolLogger)
	mempoolReactor := mempl.NewMempoolReactor(config.Mempool, mempool)
	mempoolReactor.SetLogger(mempoolLogger)

	// Make ConsensusReactor
	consensusState := consensus.NewConsensusState(config.Consensus, state.Copy(), proxyApp.Consensus(), blockStore, mempool)
	consensusState.SetLogger(consensusLogger)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := consensus.NewConsensusReactor(consensusState, fastSync)
	consensusReactor.SetLogger(consensusLogger)

	p2pLogger := logger.With("module", "p2p")

	sw := p2p.NewSwitch(config.P2P)
	sw.SetLogger(p2pLogger)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	// Optionally, start the pex reactor
	var addrBook *p2p.AddrBook
	if config.P2P.PexReactor {
		addrBook = p2p.NewAddrBook(config.P2P.AddrBookFile(), config.P2P.AddrBookStrict)
		addrBook.SetLogger(p2pLogger.With("book", config.P2P.AddrBookFile()))
		pexReactor := p2p.NewPEXReactor(addrBook)
		pexReactor.SetLogger(p2pLogger)
		sw.AddReactor("PEX", pexReactor)
	}

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
			if resQuery.Code.IsOK() {
				return nil
			}
			return errors.New(resQuery.Code.String())
		})
		sw.SetPubKeyFilter(func(pubkey crypto.PubKeyEd25519) error {
			resQuery, err := proxyApp.Query().QuerySync(abci.RequestQuery{Path: cmn.Fmt("/p2p/filter/pubkey/%X", pubkey.Bytes())})
			if err != nil {
				return err
			}
			if resQuery.Code.IsOK() {
				return nil
			}
			return errors.New(resQuery.Code.String())
		})
	}

	// add the event switch to all services
	// they should all satisfy events.Eventable
	SetEventSwitch(eventSwitch, bcReactor, mempoolReactor, consensusReactor)

	// run the profile server
	profileHost := config.ProfListenAddress
	if profileHost != "" {

		go func() {
			logger.Error("Profile server", "error", http.ListenAndServe(profileHost, nil))
		}()
	}

	node := &Node{
		config:        config,
		genesisDoc:    state.GenesisDoc,
		privValidator: privValidator,

		privKey:  privKey,
		sw:       sw,
		addrBook: addrBook,

		evsw:             eventSwitch,
		blockStore:       blockStore,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		proxyApp:         proxyApp,
		txIndexer:        txIndexer,
	}
	node.BaseService = *cmn.NewBaseService(logger, "Node", node)
	return node
}

func (n *Node) OnStart() error {
	// Create & add listener
	protocol, address := ProtocolAndAddress(n.config.P2P.ListenAddress)
	l := p2p.NewDefaultListener(protocol, address, n.config.P2P.SkipUPNP, n.Logger.With("module", "p2p"))
	n.sw.AddListener(l)

	// Start the switch
	n.sw.SetNodeInfo(n.makeNodeInfo())
	n.sw.SetNodePrivKey(n.privKey)
	_, err := n.sw.Start()
	if err != nil {
		return err
	}

	// If seeds exist, add them to the address book and dial out
	if n.config.P2P.Seeds != "" {
		// dial out
		seeds := strings.Split(n.config.P2P.Seeds, ",")
		if err := n.DialSeeds(seeds); err != nil {
			return err
		}
	}

	// Run the RPC server
	if n.config.RPC.ListenAddress != "" {
		listeners, err := n.startRPC()
		if err != nil {
			return err
		}
		n.rpcListeners = listeners
	}

	return nil
}

func (n *Node) OnStop() {
	n.BaseService.OnStop()

	n.Logger.Info("Stopping Node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()

	for _, l := range n.rpcListeners {
		n.Logger.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			n.Logger.Error("Error closing listener", "listener", l, "error", err)
		}
	}
}

func (n *Node) RunForever() {
	// Sleep forever and then...
	cmn.TrapSignal(func() {
		n.Stop()
	})
}

// Add the event switch to reactors, mempool, etc.
func SetEventSwitch(evsw types.EventSwitch, eventables ...types.Eventable) {
	for _, e := range eventables {
		e.SetEventSwitch(evsw)
	}
}

// Add a Listener to accept inbound peer connections.
// Add listeners before starting the Node.
// The first listener is the primary listener (in NodeInfo)
func (n *Node) AddListener(l p2p.Listener) {
	n.sw.AddListener(l)
}

// ConfigureRPC sets all variables in rpccore so they will serve
// rpc calls from this node
func (n *Node) ConfigureRPC() {
	rpccore.SetEventSwitch(n.evsw)
	rpccore.SetBlockStore(n.blockStore)
	rpccore.SetConsensusState(n.consensusState)
	rpccore.SetMempool(n.mempoolReactor.Mempool)
	rpccore.SetSwitch(n.sw)
	rpccore.SetPubKey(n.privValidator.PubKey)
	rpccore.SetGenesisDoc(n.genesisDoc)
	rpccore.SetAddrBook(n.addrBook)
	rpccore.SetProxyAppQuery(n.proxyApp.Query())
	rpccore.SetTxIndexer(n.txIndexer)
	rpccore.SetLogger(n.Logger.With("module", "rpc"))
}

func (n *Node) startRPC() ([]net.Listener, error) {
	n.ConfigureRPC()
	listenAddrs := strings.Split(n.config.RPC.ListenAddress, ",")

	if n.config.RPC.Unsafe {
		rpccore.AddUnsafeRoutes()
	}

	// we may expose the rpc over both a unix and tcp socket
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		wm := rpcserver.NewWebsocketManager(rpccore.Routes, n.evsw)
		rpcLogger := n.Logger.With("module", "rpc-server")
		wm.SetLogger(rpcLogger)
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

func (n *Node) Switch() *p2p.Switch {
	return n.sw
}

func (n *Node) BlockStore() *bc.BlockStore {
	return n.blockStore
}

func (n *Node) ConsensusState() *consensus.ConsensusState {
	return n.consensusState
}

func (n *Node) ConsensusReactor() *consensus.ConsensusReactor {
	return n.consensusReactor
}

func (n *Node) MempoolReactor() *mempl.MempoolReactor {
	return n.mempoolReactor
}

func (n *Node) EventSwitch() types.EventSwitch {
	return n.evsw
}

// XXX: for convenience
func (n *Node) PrivValidator() *types.PrivValidator {
	return n.privValidator
}

func (n *Node) GenesisDoc() *types.GenesisDoc {
	return n.genesisDoc
}

func (n *Node) ProxyApp() proxy.AppConns {
	return n.proxyApp
}

func (n *Node) makeNodeInfo() *p2p.NodeInfo {
	txIndexerStatus := "on"
	if _, ok := n.txIndexer.(*null.TxIndex); ok {
		txIndexerStatus = "off"
	}

	nodeInfo := &p2p.NodeInfo{
		PubKey:  n.privKey.PubKey().Unwrap().(crypto.PubKeyEd25519),
		Moniker: n.config.Moniker,
		Network: n.consensusState.GetState().ChainID,
		Version: version.Version,
		Other: []string{
			cmn.Fmt("wire_version=%v", wire.Version),
			cmn.Fmt("p2p_version=%v", p2p.Version),
			cmn.Fmt("consensus_version=%v", consensus.Version),
			cmn.Fmt("rpc_version=%v/%v", rpc.Version, rpccore.Version),
			cmn.Fmt("tx_index=%v", txIndexerStatus),
		},
	}

	// include git hash in the nodeInfo if available
	// TODO: use ld-flags
	/*if rev, err := cmn.ReadFile(n.config.GetString("revision_file")); err == nil {
		nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("revision=%v", string(rev)))
	}*/

	if !n.sw.IsListening() {
		return nodeInfo
	}

	p2pListener := n.sw.Listeners()[0]
	p2pHost := p2pListener.ExternalAddress().IP.String()
	p2pPort := p2pListener.ExternalAddress().Port
	rpcListenAddr := n.config.RPC.ListenAddress

	// We assume that the rpcListener has the same ExternalAddress.
	// This is probably true because both P2P and RPC listeners use UPnP,
	// except of course if the rpc is only bound to localhost
	nodeInfo.ListenAddr = cmn.Fmt("%v:%v", p2pHost, p2pPort)
	nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("rpc_addr=%v", rpcListenAddr))
	return nodeInfo
}

//------------------------------------------------------------------------------

func (n *Node) NodeInfo() *p2p.NodeInfo {
	return n.sw.NodeInfo()
}

func (n *Node) DialSeeds(seeds []string) error {
	return n.sw.DialSeeds(n.addrBook, seeds)
}

// Defaults to tcp
func ProtocolAndAddress(listenAddr string) (string, string) {
	protocol, address := "tcp", listenAddr
	parts := strings.SplitN(address, "://", 2)
	if len(parts) == 2 {
		protocol, address = parts[0], parts[1]
	}
	return protocol, address
}

//------------------------------------------------------------------------------
