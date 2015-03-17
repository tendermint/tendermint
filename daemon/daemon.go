package daemon

import (
	"os"
	"os/signal"

	blk "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/consensus"
	dbm "github.com/tendermint/tendermint/db"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc"
	sm "github.com/tendermint/tendermint/state"
)

type Node struct {
	lz               []p2p.Listener
	sw               *p2p.Switch
	book             *p2p.AddrBook
	pexReactor       *p2p.PEXReactor
	blockStore       *blk.BlockStore
	mempoolReactor   *mempl.MempoolReactor
	consensusState   *consensus.ConsensusState
	consensusReactor *consensus.ConsensusReactor
	privValidator    *sm.PrivValidator
}

func NewNode() *Node {
	// Get BlockStore
	blockStoreDB := dbm.GetDB("blockstore")
	blockStore := blk.NewBlockStore(blockStoreDB)

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

	// Get MempoolReactor
	mempool := mempl.NewMempool(state.Copy())
	mempoolReactor := mempl.NewMempoolReactor(mempool)

	// Get ConsensusReactor
	consensusState := consensus.NewConsensusState(state, blockStore, mempoolReactor)
	consensusReactor := consensus.NewConsensusReactor(consensusState, blockStore)
	if privValidator != nil {
		consensusReactor.SetPrivValidator(privValidator)
	}

	sw := p2p.NewSwitch([]p2p.Reactor{pexReactor, mempoolReactor, consensusReactor})
	sw.SetChainId(state.Hash(), config.App().GetString("Network"))

	return &Node{
		sw:               sw,
		book:             book,
		pexReactor:       pexReactor,
		blockStore:       blockStore,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		privValidator:    privValidator,
	}
}

func (n *Node) Start() {
	log.Info("Starting Node")
	for _, l := range n.lz {
		go n.inboundConnectionRoutine(l)
	}
	n.book.Start()
	n.sw.Start()
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
	n.lz = append(n.lz, l)
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

func Daemon() {

	// Create & start node
	n := NewNode()
	l := p2p.NewDefaultListener("tcp", config.App().GetString("ListenAddr"), false)
	n.AddListener(l)
	n.Start()

	// If seedNode is provided by config, dial out.
	if config.App().GetString("SeedNode") != "" {
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

	// Run the RPC server.
	if config.App().GetString("RPC.HTTP.ListenAddr") != "" {
		rpc.SetRPCBlockStore(n.blockStore)
		rpc.SetRPCConsensusState(n.consensusState)
		rpc.SetRPCMempoolReactor(n.mempoolReactor)
		rpc.SetRPCSwitch(n.sw)
		rpc.StartHTTPServer()
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
