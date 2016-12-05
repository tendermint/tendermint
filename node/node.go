package node

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-rpc"
	"github.com/tendermint/go-rpc/server"
	"github.com/tendermint/go-wire"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	grpccore "github.com/tendermint/tendermint/rpc/grpc"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

import _ "net/http/pprof"

type Node struct {
	config           cfg.Config
	sw               *p2p.Switch
	evsw             types.EventSwitch
	blockStore       *bc.BlockStore
	bcReactor        *bc.BlockchainReactor
	mempoolReactor   *mempl.MempoolReactor
	consensusState   *consensus.ConsensusState
	consensusReactor *consensus.ConsensusReactor
	privValidator    *types.PrivValidator
	genesisDoc       *types.GenesisDoc
	privKey          crypto.PrivKeyEd25519
	proxyApp         proxy.AppConns
}

func NewNodeDefault(config cfg.Config) *Node {
	// Get PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)
	return NewNode(config, privValidator, proxy.DefaultClientCreator(config))
}

func NewNode(config cfg.Config, privValidator *types.PrivValidator, clientCreator proxy.ClientCreator) *Node {

	// Get BlockStore
	blockStoreDB := dbm.NewDB("blockstore", config.GetString("db_backend"), config.GetString("db_dir"))
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State db
	stateDB := dbm.NewDB("state", config.GetString("db_backend"), config.GetString("db_dir"))

	// Get State
	state := sm.GetState(config, stateDB)

	// Create the proxyApp, which manages connections (consensus, mempool, query)
	proxyApp := proxy.NewAppConns(config, clientCreator, sm.NewHandshaker(config, state, blockStore))
	if _, err := proxyApp.Start(); err != nil {
		Exit(Fmt("Error starting proxy app connections: %v", err))
	}

	// add the chainid and number of validators to the global config
	config.Set("chain_id", state.ChainID)
	config.Set("num_vals", state.Validators.Size())

	// Generate node PrivKey
	privKey := crypto.GenPrivKeyEd25519()

	// Make event switch
	eventSwitch := types.NewEventSwitch()
	_, err := eventSwitch.Start()
	if err != nil {
		Exit(Fmt("Failed to start switch: %v", err))
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
	bcReactor := bc.NewBlockchainReactor(config, state.Copy(), proxyApp.Consensus(), blockStore, fastSync)

	// Make MempoolReactor
	mempool := mempl.NewMempool(config, proxyApp.Mempool())
	mempoolReactor := mempl.NewMempoolReactor(config, mempool)

	// Make ConsensusReactor
	consensusState := consensus.NewConsensusState(config, state.Copy(), proxyApp.Consensus(), blockStore, mempool)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := consensus.NewConsensusReactor(consensusState, fastSync)

	// Make p2p network switch
	sw := p2p.NewSwitch(config.GetConfig("p2p"))
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	// filter peers by addr or pubkey with a tmsp query.
	// if the query return code is OK, add peer
	// XXX: query format subject to change
	if config.GetBool("filter_peers") {
		// NOTE: addr is ip:port
		sw.SetAddrFilter(func(addr net.Addr) error {
			res := proxyApp.Query().QuerySync([]byte(Fmt("p2p/filter/addr/%s", addr.String())))
			if res.IsOK() {
				return nil
			}
			return res
		})
		sw.SetPubKeyFilter(func(pubkey crypto.PubKeyEd25519) error {
			res := proxyApp.Query().QuerySync([]byte(Fmt("p2p/filter/pubkey/%X", pubkey.Bytes())))
			if res.IsOK() {
				return nil
			}
			return res
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

	return &Node{
		config:           config,
		sw:               sw,
		evsw:             eventSwitch,
		blockStore:       blockStore,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		privValidator:    privValidator,
		genesisDoc:       state.GenesisDoc,
		privKey:          privKey,
		proxyApp:         proxyApp,
	}
}

// Call Start() after adding the listeners.
func (n *Node) Start() error {
	n.sw.SetNodeInfo(makeNodeInfo(n.config, n.sw, n.privKey))
	n.sw.SetNodePrivKey(n.privKey)
	_, err := n.sw.Start()
	return err
}

func (n *Node) Stop() {
	log.Notice("Stopping Node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()
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
	log.Notice(Fmt("Added %v", l))
	n.sw.AddListener(l)
}

func (n *Node) StartRPC() ([]net.Listener, error) {
	rpccore.SetConfig(n.config)

	rpccore.SetEventSwitch(n.evsw)
	rpccore.SetBlockStore(n.blockStore)
	rpccore.SetConsensusState(n.consensusState)
	rpccore.SetMempool(n.mempoolReactor.Mempool)
	rpccore.SetSwitch(n.sw)
	rpccore.SetPubKey(n.privValidator.PubKey)
	rpccore.SetGenesisDoc(n.genesisDoc)
	rpccore.SetProxyAppQuery(n.proxyApp.Query())

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

func makeNodeInfo(config cfg.Config, sw *p2p.Switch, privKey crypto.PrivKeyEd25519) *p2p.NodeInfo {

	nodeInfo := &p2p.NodeInfo{
		PubKey:  privKey.PubKey().(crypto.PubKeyEd25519),
		Moniker: config.GetString("moniker"),
		Network: config.GetString("chain_id"),
		Version: version.Version,
		Other: []string{
			Fmt("wire_version=%v", wire.Version),
			Fmt("p2p_version=%v", p2p.Version),
			Fmt("consensus_version=%v", consensus.Version),
			Fmt("rpc_version=%v/%v", rpc.Version, rpccore.Version),
		},
	}

	// include git hash in the nodeInfo if available
	if rev, err := ReadFile(config.GetString("revision_file")); err == nil {
		nodeInfo.Other = append(nodeInfo.Other, Fmt("revision=%v", string(rev)))
	}

	if !sw.IsListening() {
		return nodeInfo
	}

	p2pListener := sw.Listeners()[0]
	p2pHost := p2pListener.ExternalAddress().IP.String()
	p2pPort := p2pListener.ExternalAddress().Port
	rpcListenAddr := config.GetString("rpc_laddr")

	// We assume that the rpcListener has the same ExternalAddress.
	// This is probably true because both P2P and RPC listeners use UPnP,
	// except of course if the rpc is only bound to localhost
	nodeInfo.ListenAddr = Fmt("%v:%v", p2pHost, p2pPort)
	nodeInfo.Other = append(nodeInfo.Other, Fmt("rpc_addr=%v", rpcListenAddr))
	return nodeInfo
}

//------------------------------------------------------------------------------

// Users wishing to:
//	* use an external signer for their validators
//	* supply an in-proc tmsp app
// should fork tendermint/tendermint and implement RunNode to
// call NewNode with their custom priv validator and/or custom
// proxy.ClientCreator interface
func RunNode(config cfg.Config) {
	// Wait until the genesis doc becomes available
	genDocFile := config.GetString("genesis_file")
	if !FileExists(genDocFile) {
		log.Notice(Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if !FileExists(genDocFile) {
				continue
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				Exit(Fmt("Couldn't read GenesisDoc file: %v", err))
			}
			genDoc := types.GenesisDocFromJSON(jsonBlob)
			if genDoc.ChainID == "" {
				PanicSanity(Fmt("Genesis doc %v must include non-empty chain_id", genDocFile))
			}
			config.Set("chain_id", genDoc.ChainID)
		}
	}

	// Create & start node
	n := NewNodeDefault(config)

	protocol, address := ProtocolAndAddress(config.GetString("node_laddr"))
	l := p2p.NewDefaultListener(protocol, address, config.GetBool("skip_upnp"))
	n.AddListener(l)
	err := n.Start()
	if err != nil {
		Exit(Fmt("Failed to start node: %v", err))
	}

	log.Notice("Started node", "nodeInfo", n.sw.NodeInfo())

	// If seedNode is provided by config, dial out.
	if config.GetString("seeds") != "" {
		seeds := strings.Split(config.GetString("seeds"), ",")
		n.sw.DialSeeds(seeds)
	}

	// Run the RPC server.
	if config.GetString("rpc_laddr") != "" {
		_, err := n.StartRPC()
		if err != nil {
			PanicCrisis(err)
		}
	}

	// Sleep forever and then...
	TrapSignal(func() {
		n.Stop()
	})
}

func (n *Node) NodeInfo() *p2p.NodeInfo {
	return n.sw.NodeInfo()
}

func (n *Node) DialSeeds(seeds []string) {
	n.sw.DialSeeds(seeds)
}

//------------------------------------------------------------------------------
// replay

// convenience for replay mode
func newConsensusState(config cfg.Config) *consensus.ConsensusState {
	// Get BlockStore
	blockStoreDB := dbm.NewDB("blockstore", config.GetString("db_backend"), config.GetString("db_dir"))
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.NewDB("state", config.GetString("db_backend"), config.GetString("db_dir"))
	state := sm.MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))

	// Create proxyAppConn connection (consensus, mempool, query)
	proxyApp := proxy.NewAppConns(config, proxy.DefaultClientCreator(config), sm.NewHandshaker(config, state, blockStore))
	_, err := proxyApp.Start()
	if err != nil {
		Exit(Fmt("Error starting proxy app conns: %v", err))
	}

	// add the chainid to the global config
	config.Set("chain_id", state.ChainID)

	// Make event switch
	eventSwitch := types.NewEventSwitch()
	if _, err := eventSwitch.Start(); err != nil {
		Exit(Fmt("Failed to start event switch: %v", err))
	}

	mempool := mempl.NewMempool(config, proxyApp.Mempool())

	consensusState := consensus.NewConsensusState(config, state.Copy(), proxyApp.Consensus(), blockStore, mempool)
	consensusState.SetEventSwitch(eventSwitch)
	return consensusState
}

func RunReplayConsole(config cfg.Config, walFile string) {
	consensusState := newConsensusState(config)

	if err := consensusState.ReplayConsole(walFile); err != nil {
		Exit(Fmt("Error during consensus replay: %v", err))
	}
}

func RunReplay(config cfg.Config, walFile string) {
	consensusState := newConsensusState(config)

	if err := consensusState.ReplayMessages(walFile); err != nil {
		Exit(Fmt("Error during consensus replay: %v", err))
	}
	log.Notice("Replay run successfully")
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
