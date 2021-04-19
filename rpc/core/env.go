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
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
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

var (
	// set by Node
	env *Environment
)

// SetEnvironment sets up the given Environment.
// It will race if multiple Node call SetEnvironment.
func SetEnvironment(e *Environment) {
	env = e
}

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
	AddUnconditionalPeerIDs([]string) error
	AddPrivatePeerIDs([]string) error
	DialPeersAsync([]string) error
	Peers() p2p.IPeerSet
}

//----------------------------------------------
// Environment contains objects and interfaces used by the RPC. It is expected
// to be setup once during startup.
type Environment struct {
	// external, thread safe interfaces
	ProxyAppQuery   proxy.AppConnQuery
	ProxyAppMempool proxy.AppConnMempool

	// interfaces defined in types and above
	StateStore     sm.Store
	BlockStore     sm.BlockStore
	EvidencePool   sm.EvidencePool
	ConsensusState Consensus
	P2PPeers       peers
	P2PTransport   transport

	// objects
	PubKey           crypto.PubKey
	GenDoc           *types.GenesisDoc // cache the genesis structure
	TxIndexer        txindex.TxIndexer
	BlockIndexer     indexer.BlockIndexer
	ConsensusReactor *consensus.Reactor
	EventBus         *types.EventBus // thread safe
	Mempool          mempl.Mempool

	Logger log.Logger

	Config cfg.RPCConfig
}

//----------------------------------------------

func validatePage(pagePtr *int, perPage, totalCount int) (int, error) {
	// this can only happen if we haven't first run validatePerPage
	if perPage < 1 {
		panic(fmt.Errorf("%w (%d)", ctypes.ErrZeroOrNegativePerPage, perPage))
	}

	if pagePtr == nil { // no page parameter
		return 1, nil
	}

	pages := ((totalCount - 1) / perPage) + 1
	if pages == 0 {
		pages = 1 // one page (even if it's empty)
	}
	page := *pagePtr
	if page <= 0 || page > pages {
		return 1, fmt.Errorf("%w expected range: [1, %d], given %d", ctypes.ErrPageOutOfRange, pages, page)
	}

	return page, nil
}

func validatePerPage(perPagePtr *int) int {
	if perPagePtr == nil { // no per_page parameter
		return defaultPerPage
	}

	perPage := *perPagePtr
	if perPage < 1 {
		return defaultPerPage
		// in unsafe mode there is no max on the page size but in safe mode
		// we cap it to maxPerPage
	} else if perPage > maxPerPage && !env.Config.Unsafe {
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

// latestHeight can be either latest committed or uncommitted (+1) height.
func getHeight(latestHeight int64, heightPtr *int64) (int64, error) {
	if heightPtr != nil {
		height := *heightPtr
		if height <= 0 {
			return 0, fmt.Errorf("%w (requested height: %d)", ctypes.ErrZeroOrNegativeHeight, height)
		}
		if height > latestHeight {
			return 0, fmt.Errorf("%w (requested height: %d, blockchain height: %d)",
				ctypes.ErrHeightExceedsChainHead, height, latestHeight)
		}
		base := env.BlockStore.Base()
		if height < base {
			return 0, fmt.Errorf("%w (requested height: %d, base height: %d)", ctypes.ErrHeightNotAvailable, height, base)
		}
		return height, nil
	}
	return latestHeight, nil
}

func latestUncommittedHeight() int64 {
	nodeIsSyncing := env.ConsensusReactor.WaitSync()
	if nodeIsSyncing {
		return env.BlockStore.Height()
	}
	return env.BlockStore.Height() + 1
}
