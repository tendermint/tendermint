package core

import (
	"time"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

const (
	// see README
	defaultPerPage = 30
	maxPerPage     = 100

	// SubscribeTimeout is the maximum time we wait to subscribe for an event.
	// must be less than the server's write timeout (see rpcserver.DefaultConfig)
	SubscribeTimeout = 5 * time.Second
)

//----------------------------------------------
// These interfaces are used by RPC and must be thread safe

type Consensus interface {
	GetState() sm.State
	GetValidators() (int64, []*types.Validator)
	GetLastHeight() int64
	GetRoundStateJSON() ([]byte, error)
	GetRoundStateSimpleJSON() ([]byte, error)
}

type transport interface {
	Listeners() []string
	IsListening() bool
	NodeInfo() p2p.NodeInfo
}

type peers interface {
	DialPeersAsync(p2p.AddrBook, []string, bool) error
	NumPeers() (outbound, inbound, dialig int)
	Peers() p2p.IPeerSet
}

// Mempool extends the standard Mempool interface to allow getting
// total txs size. ReapMaxTxs is used by UnconfirmedTxs to reap N transactions.
type Mempool interface {
	mempl.Mempool
	ReapMaxTxs(n int) types.Txs
	TxsBytes() int64
}

//----------------------------------------------
// These package level globals come with setters
// that are expected to be called only once, on startup

var (
	// external, thread safe interfaces
	proxyAppQuery proxy.AppConnQuery

	// interfaces defined in types and above
	stateDB        dbm.DB
	blockStore     sm.BlockStore
	evidencePool   sm.EvidencePool
	consensusState Consensus
	p2pPeers       peers
	p2pTransport   transport

	// objects
	pubKey           crypto.PubKey
	genDoc           *types.GenesisDoc // cache the genesis structure
	addrBook         p2p.AddrBook
	txIndexer        txindex.TxIndexer
	consensusReactor *consensus.ConsensusReactor
	eventBus         *types.EventBus // thread safe
	mempool          Mempool

	logger log.Logger

	config cfg.RPCConfig
)

func SetStateDB(db dbm.DB) {
	stateDB = db
}

func SetBlockStore(bs sm.BlockStore) {
	blockStore = bs
}

func SetMempool(mem Mempool) {
	mempool = mem
}

func SetEvidencePool(evpool sm.EvidencePool) {
	evidencePool = evpool
}

func SetConsensusState(cs Consensus) {
	consensusState = cs
}

func SetP2PPeers(p peers) {
	p2pPeers = p
}

func SetP2PTransport(t transport) {
	p2pTransport = t
}

func SetPubKey(pk crypto.PubKey) {
	pubKey = pk
}

func SetGenesisDoc(doc *types.GenesisDoc) {
	genDoc = doc
}

func SetAddrBook(book p2p.AddrBook) {
	addrBook = book
}

func SetProxyAppQuery(appConn proxy.AppConnQuery) {
	proxyAppQuery = appConn
}

func SetTxIndexer(indexer txindex.TxIndexer) {
	txIndexer = indexer
}

func SetConsensusReactor(conR *consensus.ConsensusReactor) {
	consensusReactor = conR
}

func SetLogger(l log.Logger) {
	logger = l
}

func SetEventBus(b *types.EventBus) {
	eventBus = b
}

// SetConfig sets an RPCConfig.
func SetConfig(c cfg.RPCConfig) {
	config = c
}

func validatePage(page, perPage, totalCount int) int {
	if perPage < 1 {
		return 1
	}

	pages := ((totalCount - 1) / perPage) + 1
	if page < 1 {
		page = 1
	} else if page > pages {
		page = pages
	}

	return page
}

func validatePerPage(perPage int) int {
	if perPage < 1 {
		return defaultPerPage
	} else if perPage > maxPerPage {
		return maxPerPage
	}
	return perPage
}

func validateSkipCount(page, perPage int) int {
	skipCount := (page - 1) * perPage
	if skipCount < 0 {
		return 0
	}

	return skipCount
}
