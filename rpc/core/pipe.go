package core

import (
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-p2p"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------
// Interfaces for use by RPC
// NOTE: these methods must be thread safe!

type BlockStore interface {
	Height() int
	LoadBlockMeta(height int) *types.BlockMeta
	LoadBlock(height int) *types.Block
	LoadSeenCommit(height int) *types.Commit
	LoadBlockCommit(height int) *types.Commit
}

type Consensus interface {
	GetValidators() (int, []*types.Validator)
	GetRoundState() *consensus.RoundState
}

type Mempool interface {
	Size() int
	CheckTx(types.Tx, func(*abci.Response)) error
	Reap(int) []types.Tx
	Flush()
}

type P2P interface {
	Listeners() []p2p.Listener
	Peers() p2p.IPeerSet
	NumPeers() (outbound, inbound, dialig int)
	NodeInfo() *p2p.NodeInfo
	IsListening() bool
	DialSeeds([]string)
}

var (
	// external, thread safe interfaces
	eventSwitch   types.EventSwitch
	proxyAppQuery proxy.AppConnQuery
	config        cfg.Config

	// interfaces defined above
	blockStore     BlockStore
	consensusState Consensus
	mempool        Mempool
	p2pSwitch      P2P

	// objects
	pubKey crypto.PubKey
	genDoc *types.GenesisDoc // cache the genesis structure
)

func SetConfig(c cfg.Config) {
	config = c
}

func SetEventSwitch(evsw types.EventSwitch) {
	eventSwitch = evsw
}

func SetBlockStore(bs BlockStore) {
	blockStore = bs
}

func SetConsensusState(cs Consensus) {
	consensusState = cs
}

func SetMempool(mem Mempool) {
	mempool = mem
}

func SetSwitch(sw P2P) {
	p2pSwitch = sw
}

func SetPubKey(pk crypto.PubKey) {
	pubKey = pk
}

func SetGenesisDoc(doc *types.GenesisDoc) {
	genDoc = doc
}

func SetProxyAppQuery(appConn proxy.AppConnQuery) {
	proxyAppQuery = appConn
}
