package node

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	acm "github.com/eris-ltd/tendermint/account"
	bc "github.com/eris-ltd/tendermint/blockchain"
	. "github.com/eris-ltd/tendermint/common"
	"github.com/eris-ltd/tendermint/consensus"
	dbm "github.com/eris-ltd/tendermint/db"
	"github.com/eris-ltd/tendermint/events"
	mempl "github.com/eris-ltd/tendermint/mempool"
	"github.com/eris-ltd/tendermint/p2p"
	"github.com/eris-ltd/tendermint/rpc"
	"github.com/eris-ltd/tendermint/rpc/core"
	"github.com/eris-ltd/tendermint/rpc/server"
	sm "github.com/eris-ltd/tendermint/state"
	stypes "github.com/eris-ltd/tendermint/state/types"
	"github.com/eris-ltd/tendermint/types"
	"github.com/eris-ltd/tendermint/vm"
	"github.com/eris-ltd/tendermint/wire"
)

import _ "net/http/pprof"

type Node struct {
	sw               *p2p.Switch
	evsw             *events.EventSwitch
	book             *p2p.AddrBook
	blockStore       *bc.BlockStore
	pexReactor       *p2p.PEXReactor
	bcReactor        *bc.BlockchainReactor
	mempoolReactor   *mempl.MempoolReactor
	consensusState   *consensus.ConsensusState
	consensusReactor *consensus.ConsensusReactor
	privValidator    *types.PrivValidator
	genDoc           *stypes.GenesisDoc
	privKey          acm.PrivKeyEd25519
}

func NewNodeDefaultPrivVal() *Node {
	// Get PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)

	return NewNode(privValidator)
}

func NewNode(privValidator *types.PrivValidator) *Node {
	// Get BlockStore
	blockStoreDB := dbm.GetDB("blockstore")
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.GetDB("state")
	state := sm.LoadState(stateDB)
	var genDoc *stypes.GenesisDoc
	if state == nil {
		genDoc, state = sm.MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))
		state.Save()
		// write the gendoc to db
		buf, n, err := new(bytes.Buffer), new(int64), new(error)
		wire.WriteJSON(genDoc, buf, n, err)
		stateDB.Set(stypes.GenDocKey, buf.Bytes())
		if *err != nil {
			Exit(Fmt("Unable to write gendoc to db: %v", *err))
		}
	} else {
		genDocBytes := stateDB.Get(stypes.GenDocKey)
		err := new(error)
		wire.ReadJSONPtr(&genDoc, genDocBytes, err)
		if *err != nil {
			Exit(Fmt("Unable to read gendoc from db: %v", *err))
		}
	}
	// add the chainid to the global config
	config.Set("chain_id", state.ChainID)

	// Generate node PrivKey
	privKey := acm.GenPrivKeyEd25519()

	// Make event switch
	eventSwitch := events.NewEventSwitch()
	_, err := eventSwitch.Start()
	if err != nil {
		Exit(Fmt("Failed to start switch: %v", err))
	}

	// Make PEXReactor
	book := p2p.NewAddrBook(config.GetString("addrbook_file"))
	pexReactor := p2p.NewPEXReactor(book)

	// Make BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state.Copy(), blockStore, config.GetBool("fast_sync"))

	// Make MempoolReactor
	mempool := mempl.NewMempool(state.Copy())
	mempoolReactor := mempl.NewMempoolReactor(mempool)

	// Make ConsensusReactor
	consensusState := consensus.NewConsensusState(state.Copy(), blockStore, mempoolReactor)
	consensusReactor := consensus.NewConsensusReactor(consensusState, blockStore, config.GetBool("fast_sync"))
	if privValidator != nil {
		consensusReactor.SetPrivValidator(privValidator)
	}

	// Make p2p network switch
	sw := p2p.NewSwitch()
	sw.AddReactor("PEX", pexReactor)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	// add the event switch to all services
	// they should all satisfy events.Eventable
	SetFireable(eventSwitch, pexReactor, bcReactor, mempoolReactor, consensusReactor)

	// run the profile server
	profileHost := config.GetString("prof_laddr")
	if profileHost != "" {
		go func() {
			log.Warn("Profile server", "error", http.ListenAndServe(profileHost, nil))
		}()
	}

	// set vm log level
	vm.SetDebug(config.GetBool("vm_log"))

	return &Node{
		sw:               sw,
		evsw:             eventSwitch,
		book:             book,
		blockStore:       blockStore,
		pexReactor:       pexReactor,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		privValidator:    privValidator,
		genDoc:           genDoc,
		privKey:          privKey,
	}
}

// Call Start() after adding the listeners.
func (n *Node) Start() error {
	n.book.Start()
	n.sw.SetNodeInfo(makeNodeInfo(n.sw, n.privKey))
	n.sw.SetNodePrivKey(n.privKey)
	_, err := n.sw.Start()
	return err
}

func (n *Node) Stop() {
	log.Notice("Stopping Node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()
	n.book.Stop()
}

// Add the event switch to reactors, mempool, etc.
func SetFireable(evsw *events.EventSwitch, eventables ...events.Eventable) {
	for _, e := range eventables {
		e.SetFireable(evsw)
	}
}

// Add a Listener to accept inbound peer connections.
// Add listeners before starting the Node.
// The first listener is the primary listener (in NodeInfo)
func (n *Node) AddListener(l p2p.Listener) {
	log.Notice(Fmt("Added %v", l))
	n.sw.AddListener(l)
	n.book.AddOurAddress(l.ExternalAddress())
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
		//n.book.MarkAttempt(addr)
		return
	} else {
		log.Notice("Connected to seed", "peer", peer)
		n.book.AddAddress(addr, addr)
	}
}

func (n *Node) StartRPC() (net.Listener, error) {
	core.SetBlockStore(n.blockStore)
	core.SetConsensusState(n.consensusState)
	core.SetConsensusReactor(n.consensusReactor)
	core.SetMempoolReactor(n.mempoolReactor)
	core.SetSwitch(n.sw)
	core.SetPrivValidator(n.privValidator)
	core.SetGenDoc(n.genDoc)

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

func makeNodeInfo(sw *p2p.Switch, privKey acm.PrivKeyEd25519) *types.NodeInfo {

	nodeInfo := &types.NodeInfo{
		PubKey:  privKey.PubKey().(acm.PubKeyEd25519),
		Moniker: config.GetString("moniker"),
		ChainID: config.GetString("chain_id"),
		Version: types.Versions{
			Tendermint: Version,
			P2P:        p2p.Version,
			RPC:        rpc.Version,
			Wire:       wire.Version,
		},
	}

	// include git hash in the nodeInfo if available
	if rev, err := ReadFile(config.GetString("revision_file")); err == nil {
		nodeInfo.Version.Revision = string(rev)
	}

	if !sw.IsListening() {
		return nodeInfo
	}

	p2pListener := sw.Listeners()[0]
	p2pHost := p2pListener.ExternalAddress().IP.String()
	p2pPort := p2pListener.ExternalAddress().Port
	rpcListenAddr := config.GetString("rpc_laddr")
	_, rpcPortStr, _ := net.SplitHostPort(rpcListenAddr)
	rpcPort, err := strconv.Atoi(rpcPortStr)
	if err != nil {
		PanicSanity(Fmt("Expected numeric RPC.ListenAddr port but got %v", rpcPortStr))
	}

	// We assume that the rpcListener has the same ExternalAddress.
	// This is probably true because both P2P and RPC listeners use UPnP,
	// except of course if the rpc is only bound to localhost
	nodeInfo.Host = p2pHost
	nodeInfo.P2PPort = p2pPort
	nodeInfo.RPCPort = uint16(rpcPort)
	return nodeInfo
}

//------------------------------------------------------------------------------

func RunNode() {

	// Wait until the genesis doc becomes available
	genDocFile := config.GetString("genesis_file")
	if !FileExists(genDocFile) {
		log.Notice(Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if FileExists(genDocFile) {
				break
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				Exit(Fmt("Couldn't read GenesisDoc file: %v", err))
			}
			genDoc := stypes.GenesisDocFromJSON(jsonBlob)
			if genDoc.ChainID == "" {
				PanicSanity(Fmt("Genesis doc %v must include non-empty chain_id", genDocFile))
			}
			config.Set("chain_id", genDoc.ChainID)
			config.Set("genesis_doc", genDoc)
		}
	}

	// Create & start node
	n := NewNodeDefaultPrivVal()
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"))
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
