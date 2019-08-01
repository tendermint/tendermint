package core

import (
	"fmt"
	"time"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
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
	AddPersistentPeers([]string) error
	DialPeersAsync([]string) error
	NumPeers() (outbound, inbound, dialig int)
	Peers() p2p.IPeerSet
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
	mempool          mempl.Mempool

	logger log.Logger

	config cfg.RPCConfig
)

func SetStateDB(db dbm.DB) {
	stateDB = db
}

func SetBlockStore(bs sm.BlockStore) {
	blockStore = bs
}

func SetMempool(mem mempl.Mempool) {
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

func validatePage(page, perPage, totalCount int) (int, error) {
	if perPage < 1 {
		panic(fmt.Sprintf("zero or negative perPage: %d", perPage))
	}

	if page == 0 {
		return 1, nil // default
	}

	pages := ((totalCount - 1) / perPage) + 1
	if pages == 0 {
		pages = 1 // one page (even if it's empty)
	}
	if page < 0 || page > pages {
		return 1, fmt.Errorf("page should be within [0, %d] range, given %d", pages, page)
	}

	return page, nil
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
