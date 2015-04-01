package daemon

import (
	"os"
	"os/signal"

	"github.com/ebuchman/debora"
	bc "github.com/tendermint/tendermint2/blockchain"
	. "github.com/tendermint/tendermint2/common"
	"github.com/tendermint/tendermint2/config"
	"github.com/tendermint/tendermint2/consensus"
	dbm "github.com/tendermint/tendermint2/db"
	mempl "github.com/tendermint/tendermint2/mempool"
	"github.com/tendermint/tendermint2/p2p"
	"github.com/tendermint/tendermint2/rpc"
	"github.com/tendermint/tendermint2/rpc/core"
	sm "github.com/tendermint/tendermint2/state"
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
	SrcPath   = "github.com/tendermint/tendermint2/cmd"
	PublicKey = "30820122300d06092a864886f70d01010105000382010f003082010a0282010100dd861e9cd5a3f3fc27d46531aa9d87f5b63f6358fa00397482c4ab93abf4ab2e3ed75380fc714d52b5e80afc184f21d5732f2d6dacc23f0e802e585ee005347c2af0ad992ee5c11b2a96f72bcae78bef314ba4448b33c3a1df7a4d6e6a808d21dfeb67ef974c0357ba54649dbcd92ec2a8d3a510da747e70cb859a7f9b15a6eceb2179c225afd3f8fb15be38988f9b82622d855f343af5830ca30a5beff3905b618f6cc39142a60ff5840595265a1f7b9fbd504760667a1b2508097c1831fd13f54c794a08468d65db9e27aff0a889665ebd7de4a6e9a6c09b3811b6cda623be48e1214ba0f9b378441e2a02b3891bc8ec1ae7081988e15c2f53fa6512784b390203010001"

	DeboraCallPort = 56565
)

type DeboraMode int

const (
	DeboraPeerMode DeboraMode = iota
	DeboraDevMode
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
