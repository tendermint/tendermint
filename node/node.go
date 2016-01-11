package node

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-rpc"
	"github.com/tendermint/go-rpc/server"
	"github.com/tendermint/go-wire"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/rpc/core"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmsp/example/golang"
)

import _ "net/http/pprof"

type Node struct {
	sw               *p2p.Switch
	evsw             *events.EventSwitch
	blockStore       *bc.BlockStore
	bcReactor        *bc.BlockchainReactor
	mempoolReactor   *mempl.MempoolReactor
	consensusState   *consensus.ConsensusState
	consensusReactor *consensus.ConsensusReactor
	privValidator    *types.PrivValidator
	genesisDoc       *types.GenesisDoc
	privKey          crypto.PrivKeyEd25519
}

func NewNode(privValidator *types.PrivValidator) *Node {
	// Get BlockStore
	blockStoreDB := dbm.GetDB("blockstore")
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	state := getState()

	// Create two proxyAppConn connections,
	// one for the consensus and one for the mempool.
	proxyAddr := config.GetString("proxy_app")
	proxyAppConnMempool := getProxyApp(proxyAddr, state.AppHash)
	proxyAppConnConsensus := getProxyApp(proxyAddr, state.AppHash)

	// add the chainid to the global config
	config.Set("chain_id", state.ChainID)

	// Generate node PrivKey
	privKey := crypto.GenPrivKeyEd25519()

	// Make event switch
	eventSwitch := events.NewEventSwitch()
	_, err := eventSwitch.Start()
	if err != nil {
		Exit(Fmt("Failed to start switch: %v", err))
	}

	// Make BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state.Copy(), proxyAppConnConsensus, blockStore, config.GetBool("fast_sync"))

	// Make MempoolReactor
	mempool := mempl.NewMempool(proxyAppConnMempool)
	mempoolReactor := mempl.NewMempoolReactor(mempool)

	// Make ConsensusReactor
	consensusState := consensus.NewConsensusState(state.Copy(), proxyAppConnConsensus, blockStore, mempool)
	consensusReactor := consensus.NewConsensusReactor(consensusState, blockStore, config.GetBool("fast_sync"))
	if privValidator != nil {
		consensusReactor.SetPrivValidator(privValidator)
	}

	// deterministic accountability
	err = consensusState.OpenFileForMessageLog(config.GetString("cs_msg_log"))
	if err != nil {
		log.Error("Failed to open cs_msg_log", "error", err.Error())
	}

	// Make p2p network switch
	sw := p2p.NewSwitch()
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

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
	}
}

// Call Start() after adding the listeners.
func (n *Node) Start() error {
	n.sw.SetNodeInfo(makeNodeInfo(n.sw, n.privKey))
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
func SetEventSwitch(evsw *events.EventSwitch, eventables ...events.Eventable) {
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

// Dial a list of seeds in random order
// Spawns a go routine for each dial
func (n *Node) DialSeed() {
	// permute the list, dial them in random order.
	seeds := strings.Split(config.GetString("seeds"), ",")
	perm := rand.Perm(len(seeds))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			time.Sleep(time.Duration(rand.Int63n(3000)) * time.Millisecond)
			j := perm[i]
			addr := p2p.NewNetAddressString(seeds[j])
			n.dialSeed(addr)
		}(i)
	}
}

func (n *Node) dialSeed(addr *p2p.NetAddress) {
	peer, err := n.sw.DialPeerWithAddress(addr)
	if err != nil {
		log.Error("Error dialing seed", "error", err)
		return
	} else {
		log.Notice("Connected to seed", "peer", peer)
	}
}

func (n *Node) StartRPC() (net.Listener, error) {
	core.SetBlockStore(n.blockStore)
	core.SetConsensusState(n.consensusState)
	core.SetConsensusReactor(n.consensusReactor)
	core.SetMempoolReactor(n.mempoolReactor)
	core.SetSwitch(n.sw)
	core.SetPrivValidator(n.privValidator)
	core.SetGenesisDoc(n.genesisDoc)

	listenAddr := config.GetString("rpc_laddr")

	mux := http.NewServeMux()
	wm := rpcserver.NewWebsocketManager(core.Routes, n.evsw)
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	rpcserver.RegisterRPCFuncs(mux, core.Routes)
	return rpcserver.StartHTTPServer(listenAddr, mux)
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

func (n *Node) MempoolReactor() *mempl.MempoolReactor {
	return n.mempoolReactor
}

func (n *Node) EventSwitch() *events.EventSwitch {
	return n.evsw
}

func makeNodeInfo(sw *p2p.Switch, privKey crypto.PrivKeyEd25519) *p2p.NodeInfo {

	nodeInfo := &p2p.NodeInfo{
		PubKey:  privKey.PubKey().(crypto.PubKeyEd25519),
		Moniker: config.GetString("moniker"),
		Network: config.GetString("chain_id"),
		Version: Version,
		Other: []string{
			Fmt("p2p_version=%v", p2p.Version),
			Fmt("rpc_version=%v", rpc.Version),
			Fmt("wire_version=%v", wire.Version),
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

// Users wishing to use an external signer for their validators
// should fork tendermint/tendermint and implement RunNode to
// load their custom priv validator and call NewNode(privVal)
func RunNode() {

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
			config.Set("genesis_doc", genDoc)
		}
	}

	// Get PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)

	// Create & start node
	n := NewNode(privValidator)
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"), config.GetBool("skip_upnp"))
	n.AddListener(l)
	err := n.Start()
	if err != nil {
		Exit(Fmt("Failed to start node: %v", err))
	}

	log.Notice("Started node", "nodeInfo", n.sw.NodeInfo())

	// If seedNode is provided by config, dial out.
	if config.GetString("seeds") != "" {
		n.DialSeed()
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

func newConsensusState() *consensus.ConsensusState {
	// Get BlockStore
	blockStoreDB := dbm.GetDB("blockstore")
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.GetDB("state")
	state := sm.MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))

	// Create two proxyAppCtx connections,
	// one for the consensus and one for the mempool.
	proxyAddr := config.GetString("proxy_app")
	proxyAppCtxMempool := getProxyApp(proxyAddr, state.LastAppHash)
	proxyAppCtxConsensus := getProxyApp(proxyAddr, state.LastAppHash)

	// add the chainid to the global config
	config.Set("chain_id", state.ChainID)

	// Make event switch
	eventSwitch := events.NewEventSwitch()
	_, err := eventSwitch.Start()
	if err != nil {
		Exit(Fmt("Failed to start event switch: %v", err))
	}

	mempool := mempl.NewMempool(proxyAppCtxMempool)

	consensusState := consensus.NewConsensusState(state.Copy(), proxyAppCtxConsensus, blockStore, mempool)
	consensusState.SetEventSwitch(eventSwitch)
	return consensusState
}

func RunReplayConsole() {
	msgLogFile := config.GetString("cs_msg_log")
	if msgLogFile == "" {
		Exit("cs_msg_log file name not set in tendermint config")
	}

	consensusState := newConsensusState()

	if err := consensusState.ReplayConsole(msgLogFile); err != nil {
		Exit(Fmt("Error during consensus replay: %v", err))
	}
}

func RunReplay() {
	msgLogFile := config.GetString("cs_msg_log")
	if msgLogFile == "" {
		Exit("cs_msg_log file name not set in tendermint config")
	}

	consensusState := newConsensusState()

	if err := consensusState.ReplayMessages(msgLogFile); err != nil {
		Exit(Fmt("Error during consensus replay: %v", err))
	}
	log.Notice("Replay run successfully")
}

// Load the most recent state from "state" db,
// or create a new one (and save) from genesis.
func getState() *sm.State {
	stateDB := dbm.GetDB("state")
	state := sm.LoadState(stateDB)
	if state == nil {
		state = sm.MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))
		state.Save()
	}
	return state
}

// Get a connection to the proxyAppConn addr.
// Check the current hash, and panic if it doesn't match.
func getProxyApp(addr string, hash []byte) (proxyAppConn proxy.AppConn) {
	// use local app (for testing)
	if addr == "local" {
		app := example.NewCounterApplication(true)
		mtx := new(sync.Mutex)
		proxyAppConn = proxy.NewLocalAppConn(mtx, app)
	} else {
		proxyConn, err := Connect(addr)
		if err != nil {
			Exit(Fmt("Failed to connect to proxy for mempool: %v", err))
		}
		remoteApp := proxy.NewRemoteAppConn(proxyConn, 1024)
		remoteApp.Start()

		proxyAppConn = remoteApp
	}

	// Check the hash
	currentHash, err := proxyAppConn.GetHashSync()
	if err != nil {
		PanicCrisis(Fmt("Error in getting proxyAppConn hash: %v", err))
	}
	if !bytes.Equal(hash, currentHash) {
		PanicCrisis(Fmt("ProxyApp hash does not match.  Expected %X, got %X", hash, currentHash))
	}

	return proxyAppConn
}
