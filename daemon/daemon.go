package daemon

import (
	"os"
	"os/signal"

	"github.com/ebuchman/debora"
	bc "github.com/tendermint/tendermint/blockchain"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/consensus"
	dbm "github.com/tendermint/tendermint/db"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc"
	"github.com/tendermint/tendermint/rpc/core"
	sm "github.com/tendermint/tendermint/state"
)

type Node struct {
	sw               *p2p.Switch
	book             *p2p.AddrBook
	blockStore       *bc.BlockStore
	pexReactor       *p2p.PEXReactor
	bcReactor        *bc.BlockchainReactor
	mempoolReactor   *mempl.MempoolReactor
	consensusState   *consensus.ConsensusState
	consensusReactor *consensus.ConsensusReactor
	privValidator    *sm.PrivValidator
}

func NewNode() *Node {
	// Get BlockStore
	blockStoreDB := dbm.GetDB("blockstore")
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.GetDB("state")
	state := sm.LoadState(stateDB)
	if state == nil {
		state = sm.MakeGenesisStateFromFile(stateDB, config.App().GetString("GenesisFile"))
		state.Save()
	}

	// Get PrivValidator
	var privValidator *sm.PrivValidator
	if _, err := os.Stat(config.App().GetString("PrivValidatorFile")); err == nil {
		privValidator = sm.LoadPrivValidator(config.App().GetString("PrivValidatorFile"))
		log.Info("Loaded PrivValidator", "file", config.App().GetString("PrivValidatorFile"), "privValidator", privValidator)
	} else {
		log.Info("No PrivValidator found", "file", config.App().GetString("PrivValidatorFile"))
	}

	// Get PEXReactor
	book := p2p.NewAddrBook(config.App().GetString("AddrBookFile"))
	pexReactor := p2p.NewPEXReactor(book)

	// Get BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state, blockStore, config.App().GetBool("FastSync"))

	// Get MempoolReactor
	mempool := mempl.NewMempool(state.Copy())
	mempoolReactor := mempl.NewMempoolReactor(mempool)

	// Get ConsensusReactor
	consensusState := consensus.NewConsensusState(state, blockStore, mempoolReactor)
	consensusReactor := consensus.NewConsensusReactor(consensusState, blockStore)
	if privValidator != nil {
		consensusReactor.SetPrivValidator(privValidator)
	}

	sw := p2p.NewSwitch()
	sw.SetNetwork(config.App().GetString("Network"))
	sw.AddReactor("PEX", pexReactor)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	return &Node{
		sw:               sw,
		book:             book,
		blockStore:       blockStore,
		pexReactor:       pexReactor,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		privValidator:    privValidator,
	}
}

func (n *Node) Start() {
	log.Info("Starting Node")
	n.book.Start()
	n.sw.Reactor("PEX").Start(n.sw)
	n.sw.Reactor("MEMPOOL").Start(n.sw)
	n.sw.Reactor("BLOCKCHAIN").Start(n.sw)
	if !config.App().GetBool("FastSync") {
		n.sw.Reactor("CONSENSUS").Start(n.sw)
	}
}

func (n *Node) Stop() {
	log.Info("Stopping Node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()
	n.book.Stop()
}

// Add a Listener to accept inbound peer connections.
func (n *Node) AddListener(l p2p.Listener) {
	log.Info(Fmt("Added %v", l))
	n.sw.AddListener(l)
	n.book.AddOurAddress(l.ExternalAddress())
}

func (n *Node) inboundConnectionRoutine(l p2p.Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}
		// New inbound connection!
		peer, err := n.sw.AddPeerWithConnection(inConn, false)
		if err != nil {
			log.Info("Ignoring error from inbound connection: %v\n%v",
				peer, err)
			continue
		}
		// NOTE: We don't yet have the external address of the
		// remote (if they have a listener at all).
		// PEXReactor's pexRoutine will handle that.
	}

	// cleanup
}

func (n *Node) DialSeed() {
	addr := p2p.NewNetAddressString(config.App().GetString("SeedNode"))
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

func (n *Node) StartRPC() {
	core.SetBlockStore(n.blockStore)
	core.SetConsensusState(n.consensusState)
	core.SetMempoolReactor(n.mempoolReactor)
	core.SetSwitch(n.sw)
	rpc.StartHTTPServer()
}

func (n *Node) Switch() *p2p.Switch {
	return n.sw
}

func (n *Node) ConsensusState() *consensus.ConsensusState {
	return n.consensusState
}

func (n *Node) MempoolReactor() *mempl.MempoolReactor {
	return n.mempoolReactor
}

//------------------------------------------------------------------------------

// debora variables
var (
	AppName   = "tendermint"
	SrcPath   = "github.com/tendermint/tendermint/cmd"
	PublicKey = "30820122300d06092a864886f70d01010105000382010f003082010a0282010100d1ffab251e05c0cae7bdd5f94c1b9644d4eb66847ee2e9a622b09e0228f2e70e6fecd1dfe6b3dc59fefab1abf0ff4e5d9657541cbe697ab1cf23fb26c9b857f6b6db8b67a0223b514ca77c8f1e049eaf9477d1a5f8041d045eeb0253a3c1ff7b90150d9b5c814a8d05fb707dd35aac118d5457334a557a82579f727a8bed521b0895b73da2458ffd1fc4be91adb624cc25731194d491f23ed47bf9a7265d28b23885e8a70625772eeeaf8e56ff3a1a2f33934668cc3a874042711f8b386da7842c117441a4d6ed29a182a00499ed5d4b6ce9532c5468d3976991f66d595a6f361e29cdf7750cf1c21e583e4c2207334c8d33ead731bf1172793b176089978b110203010001"

	DeboraCallPort = 56565
)

type DeboraMode int

const (
	DeboraNullMode = iota // debora off by default
	DeboraPeerMode        // upgradeable
	DeboraDevMode         // upgrader
)

func deboraBroadcast(n *Node) func([]byte) {
	return func(payload []byte) {
		msg := &p2p.PexDeboraMessage{Payload: payload}
		n.sw.Broadcast(p2p.PexChannel, msg)
	}
}

func Daemon(deborable DeboraMode) {
	// Add to debora
	if deborable == DeboraPeerMode {
		// TODO: support debora.logfile
		if err := debora.Add(PublicKey, SrcPath, AppName, config.App().GetString("Debora.LogFile")); err != nil {
			log.Info("Failed to add program to debora", "error", err)
		}
	}

	// Create & start node
	n := NewNode()
	l := p2p.NewDefaultListener("tcp", config.App().GetString("ListenAddr"), false)
	n.AddListener(l)
	n.Start()

	if deborable == DeboraDevMode {
		log.Info("Running debora-dev server (listen to call)")
		debora.DebListenAndServe("tendermint", DeboraCallPort, deboraBroadcast(n))
	}

	// If seedNode is provided by config, dial out.
	if config.App().GetString("SeedNode") != "" {
		n.DialSeed()
	}

	// Run the RPC server.
	if config.App().GetString("RPC.HTTP.ListenAddr") != "" {
		n.StartRPC()
	}

	// Sleep forever and then...
	trapSignal(func() {
		n.Stop()
	})
}

func trapSignal(cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Info(Fmt("captured %v, exiting..", sig))
			cb()
			os.Exit(1)
		}
	}()
	select {}
}
