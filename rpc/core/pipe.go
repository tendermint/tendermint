package core

import (
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/consensus"
	p2p "github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/log"
)

//----------------------------------------------
// These interfaces are used by RPC and must be thread safe

type Consensus interface {
	GetValidators() (int, []*types.Validator)
	GetRoundState() *consensus.RoundState
}

type P2P interface {
	Listeners() []p2p.Listener
	Peers() p2p.IPeerSet
	NumPeers() (outbound, inbound, dialig int)
	NodeInfo() *p2p.NodeInfo
	IsListening() bool
	DialSeeds(*p2p.AddrBook, []string) error
}

//----------------------------------------------

var (
	// external, thread safe interfaces
	eventSwitch   types.EventSwitch
	proxyAppQuery proxy.AppConnQuery

	// interfaces defined in types and above
	blockStore     types.BlockStore
	mempool        types.Mempool
	consensusState Consensus
	p2pSwitch      P2P

	// objects
	pubKey    crypto.PubKey
	genDoc    *types.GenesisDoc // cache the genesis structure
	addrBook  *p2p.AddrBook
	txIndexer txindex.TxIndexer

	logger log.Logger
)

func SetEventSwitch(evsw types.EventSwitch) {
	eventSwitch = evsw
}

func SetBlockStore(bs types.BlockStore) {
	blockStore = bs
}

func SetMempool(mem types.Mempool) {
	mempool = mem
}

func SetConsensusState(cs Consensus) {
	consensusState = cs
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

func SetAddrBook(book *p2p.AddrBook) {
	addrBook = book
}

func SetProxyAppQuery(appConn proxy.AppConnQuery) {
	proxyAppQuery = appConn
}

func SetTxIndexer(indexer txindex.TxIndexer) {
	txIndexer = indexer
}

func SetLogger(l log.Logger) {
	logger = l
}
