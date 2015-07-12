package node

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/tendermint/tendermint/binary"
	bc "github.com/tendermint/tendermint/blockchain"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/consensus"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/events"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc/core"
	"github.com/tendermint/tendermint/rpc/server"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

import _ "net/http/pprof"

func init() {
	go func() {
		fmt.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()
}

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
	privValidator    *sm.PrivValidator
	genDoc           *sm.GenesisDoc
}

func NewNode() *Node {
	// Get BlockStore
	blockStoreDB := dbm.GetDB("blockstore")
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.GetDB("state")
	state := sm.LoadState(stateDB)
	var genDoc *sm.GenesisDoc
	if state == nil {
		genDoc, state = sm.MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))
		state.Save()
		// write the gendoc to db
		buf, n, err := new(bytes.Buffer), new(int64), new(error)
		binary.WriteJSON(genDoc, buf, n, err)
		stateDB.Set(sm.GenDocKey, buf.Bytes())
		if *err != nil {
			panic(Fmt("Unable to write gendoc to db: %v", err))
		}
	} else {
		genDocBytes := stateDB.Get(sm.GenDocKey)
		err := new(error)
		binary.ReadJSONPtr(&genDoc, genDocBytes, err)
		if *err != nil {
			panic(Fmt("Unable to read gendoc from db: %v", err))
		}
	}
	// add the chainid to the global config
	config.Set("chain_id", state.ChainID)

	// Get PrivValidator
	var privValidator *sm.PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	if _, err := os.Stat(privValidatorFile); err == nil {
		privValidator = sm.LoadPrivValidator(privValidatorFile)
		log.Info("Loaded PrivValidator",
			"file", privValidatorFile, "privValidator", privValidator)
	} else {
		privValidator = sm.GenPrivValidator()
		privValidator.SetFile(privValidatorFile)
		privValidator.Save()
		log.Info("Generated PrivValidator", "file", privValidatorFile)
	}

	eventSwitch := new(events.EventSwitch)
	eventSwitch.Start()

	// Get PEXReactor
	book := p2p.NewAddrBook(config.GetString("addrbook_file"))
	pexReactor := p2p.NewPEXReactor(book)

	// Get BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state.Copy(), blockStore, config.GetBool("fast_sync"))

	// Get MempoolReactor
	mempool := mempl.NewMempool(state.Copy())
	mempoolReactor := mempl.NewMempoolReactor(mempool)

	// Get ConsensusReactor
	consensusState := consensus.NewConsensusState(state.Copy(), blockStore, mempoolReactor)
	consensusReactor := consensus.NewConsensusReactor(consensusState, blockStore, config.GetBool("fast_sync"))
	if privValidator != nil {
		consensusReactor.SetPrivValidator(privValidator)
	}

	sw := p2p.NewSwitch()
	sw.AddReactor("PEX", pexReactor)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	// add the event switch to all services
	// they should all satisfy events.Eventable
	SetFireable(eventSwitch, pexReactor, bcReactor, mempoolReactor, consensusReactor)

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
	}
}

// Call Start() after adding the listeners.
func (n *Node) Start() {
	log.Info("Starting Node", "chainID", config.GetString("chain_id"))
	n.book.Start()
	n.sw.SetNodeInfo(makeNodeInfo(n.sw))
	n.sw.Start()
}

func (n *Node) Stop() {
	log.Info("Stopping Node")
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
	log.Info(Fmt("Added %v", l))
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
		log.Info("Connected to seed", "peer", peer)
		n.book.AddAddress(addr, addr)
	}
}

func (n *Node) StartRPC() net.Listener {
	core.SetBlockStore(n.blockStore)
	core.SetConsensusState(n.consensusState)
	core.SetConsensusReactor(n.consensusReactor)
	core.SetMempoolReactor(n.mempoolReactor)
	core.SetSwitch(n.sw)
	core.SetPrivValidator(n.privValidator)
	core.SetGenDoc(n.genDoc)

	listenAddr := config.GetString("rpc_laddr")
	mux := http.NewServeMux()
	rpcserver.RegisterEventsHandler(mux, n.evsw)
	rpcserver.RegisterRPCFuncs(mux, core.Routes)
	listener, err := rpcserver.StartHTTPServer(listenAddr, mux)
	if err != nil {
		panic(err)
	}
	return listener
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

func makeNodeInfo(sw *p2p.Switch) *types.NodeInfo {
	nodeInfo := &types.NodeInfo{
		ChainID: config.GetString("chain_id"),
		Moniker: config.GetString("moniker"),
		Version: config.GetString("version"),
		UUID:    uuid.New(),
	}

	// include git hash in the nodeInfo if available
	if rev, err := ReadFile(path.Join(TendermintRepo, ".revision")); err == nil {
		nodeInfo.Revision = string(rev)
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
		panic(Fmt("Expected numeric RPC.ListenAddr port but got %v", rpcPortStr))
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
	// Create & start node
	n := NewNode()
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"), false)
	n.AddListener(l)
	n.Start()

	// If seedNode is provided by config, dial out.
	if config.GetString("seeds") != "" {
		n.DialSeed()
	}

	// Run the RPC server.
	if config.GetString("rpc_laddr") != "" {
		n.StartRPC()
	}

	// Sleep forever and then...
	TrapSignal(func() {
		n.Stop()
	})
}
