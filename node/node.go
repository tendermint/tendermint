package node

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/spf13/viper"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	bc "github.com/tendermint/tendermint/blockchain"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
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

	_ "net/http/pprof"
)

type Node struct {
	cmn.BaseService

	// config
	config        *viper.Viper         // user config
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

func NewNodeDefault(config *viper.Viper) *Node {
	// Get PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)
	return NewNode(config, privValidator, proxy.DefaultClientCreator(
		config.GetString("proxy_app"),
		config.GetString("abci"),
		config.GetString("db_dir"),
	))
	// config.ABCI.ProxyApp, config.ABCI.Mode, config.DB.Dir))
}

func NewNode(config *viper.Viper, privValidator *types.PrivValidator, clientCreator proxy.ClientCreator) *Node {

	tmConfig := new(tmcfg.Config)
	if err := config.Unmarshal(tmConfig); err != nil {
		panic(err)
	}

	// Get BlockStore
	blockStoreDB := dbm.NewDB("blockstore", tmConfig.DB.Backend, tmConfig.DB.Dir)
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.NewDB("state", tmConfig.DB.Backend, tmConfig.DB.Dir)
	state := sm.GetState(stateDB, tmConfig.Chain.GenesisFile)

	// add the chainid and number of validators to the global config
	config.Set("chain_id", state.ChainID)

	// Create the proxyApp, which manages connections (consensus, mempool, query)
	// and sync tendermint and the app by replaying any necessary blocks
	proxyApp := proxy.NewAppConns(clientCreator, consensus.NewHandshaker(state, blockStore))
	if _, err := proxyApp.Start(); err != nil {
		cmn.Exit(cmn.Fmt("Error starting proxy app connections: %v", err))
	}

	// reload the state (it may have been updated by the handshake)
	state = sm.LoadState(stateDB)

	// Transaction indexing
	var txIndexer txindex.TxIndexer
	switch tmConfig.DB.TxIndex {
	case "kv":
		store := dbm.NewDB("tx_index", tmConfig.DB.Backend, tmConfig.DB.Dir)
		txIndexer = kv.NewTxIndex(store)
	default:
		txIndexer = &null.TxIndex{}
	}
	state.TxIndexer = txIndexer

	// Generate node PrivKey
	privKey := crypto.GenPrivKeyEd25519()

	// Make event switch
	eventSwitch := types.NewEventSwitch()
	_, err := eventSwitch.Start()
	if err != nil {
		cmn.Exit(cmn.Fmt("Failed to start switch: %v", err))
	}

	// Decide whether to fast-sync or not
	// We don't fast-sync when the only validator is us.
	fastSync := config.GetBool("fast_sync")
	if state.Validators.Size() == 1 {
		addr, _ := state.Validators.GetByIndex(0)
		if bytes.Equal(privValidator.Address, addr) {
			fastSync = false
		}
	}

	// Make BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state.Copy(), proxyApp.Consensus(), blockStore, fastSync)

	// Make MempoolReactor
	mempool := mempl.NewMempool(mempoolConfig(config), proxyApp.Mempool())
	mempoolReactor := mempl.NewMempoolReactor(mempoolConfig(config), mempool)

	// Make ConsensusReactor
	consensusState := consensus.NewConsensusState(config, state.Copy(), proxyApp.Consensus(), blockStore, mempool)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := consensus.NewConsensusReactor(consensusState, fastSync)

	// Make p2p network switch
	p2pConfig := viper.New()
	if config.IsSet("p2p") { //TODO verify this necessary, where is this ever set?
		p2pConfig = config.Get("p2p").(*viper.Viper)
	}
	sw := p2p.NewSwitch(p2pConfig)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	// Optionally, start the pex reactor
	var addrBook *p2p.AddrBook
	if config.GetBool("pex_reactor") {
		addrBook = p2p.NewAddrBook(config.GetString("addrbook_file"), config.GetBool("addrbook_strict"))
		pexReactor := p2p.NewPEXReactor(addrBook)
		sw.AddReactor("PEX", pexReactor)
	}

	// Filter peers by addr or pubkey with an ABCI query.
	// If the query return code is OK, add peer.
	// XXX: Query format subject to change
	if config.GetBool("filter_peers") {
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
	profileHost := config.GetString("prof_laddr")
	if profileHost != "" {

		go func() {
			log.Warn("Profile server", "error", http.ListenAndServe(profileHost, nil))
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
	node.BaseService = *cmn.NewBaseService(log, "Node", node)
	return node
}

func (n *Node) OnStart() error {

	// Create & add listener
	protocol, address := ProtocolAndAddress(n.config.GetString("node_laddr"))
	l := p2p.NewDefaultListener(protocol, address, n.config.GetBool("skip_upnp"))
	n.sw.AddListener(l)

	// Start the switch
	n.sw.SetNodeInfo(n.makeNodeInfo())
	n.sw.SetNodePrivKey(n.privKey)
	_, err := n.sw.Start()
	if err != nil {
		return err
	}

	// If seeds exist, add them to the address book and dial out
	if n.config.GetString("seeds") != "" {
		// dial out
		seeds := strings.Split(n.config.GetString("seeds"), ",")
		if err := n.DialSeeds(seeds); err != nil {
			return err
		}
	}

	// Run the RPC server
	if n.config.GetString("rpc_laddr") != "" {
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

	log.Notice("Stopping Node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()

	for _, l := range n.rpcListeners {
		log.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			log.Error("Error closing listener", "listener", l, "error", err)
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
}

func (n *Node) startRPC() ([]net.Listener, error) {
	n.ConfigureRPC()
	listenAddrs := strings.Split(n.config.GetString("rpc_laddr"), ",")

	// we may expose the rpc over both a unix and tcp socket
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		wm := rpcserver.NewWebsocketManager(rpccore.Routes, n.evsw)
		mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(mux, rpccore.Routes)
		listener, err := rpcserver.StartHTTPServer(listenAddr, mux)
		if err != nil {
			return nil, err
		}
		listeners[i] = listener
	}

	// we expose a simplified api over grpc for convenience to app devs
	grpcListenAddr := n.config.GetString("grpc_laddr")
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
		Moniker: n.config.GetString("moniker"),
		Network: n.config.GetString("chain_id"),
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
	if rev, err := cmn.ReadFile(n.config.GetString("revision_file")); err == nil {
		nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("revision=%v", string(rev)))
	}

	if !n.sw.IsListening() {
		return nodeInfo
	}

	p2pListener := n.sw.Listeners()[0]
	p2pHost := p2pListener.ExternalAddress().IP.String()
	p2pPort := p2pListener.ExternalAddress().Port
	rpcListenAddr := n.config.GetString("rpc_laddr")

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

func mempoolConfig(config *viper.Viper) mempl.Config {
	return mempl.Config{
		Recheck:      config.GetBool("mempool_recheck"),
		RecheckEmpty: config.GetBool("mempool_recheck_empty"),
		Broadcast:    config.GetBool("mempool_broadcast"),
		WalDir:       config.GetString("mempool_wal_dir"),
	}
}
