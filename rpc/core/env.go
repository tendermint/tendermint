package core

import (
	"fmt"
	"time"

	dbm "github.com/tendermint/tm-db"

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
	DialPeersAsync([]string) error
	Peers() p2p.IPeerSet
}

//----------------------------------------------
// Environment contains objects and interfaces used by the RPC. It is expected
// to be setup once during startup.
type Environment struct {
	// external, thread safe interfaces
	ProxyAppQuery proxy.AppConnQuery

	// interfaces defined in types and above
	StateDB        dbm.DB
	BlockStore     sm.BlockStore
	EvidencePool   sm.EvidencePool
	ConsensusState Consensus
	P2PPeers       peers
	P2PTransport   transport

	// objects
	PubKey           crypto.PubKey
	GenDoc           *types.GenesisDoc // cache the genesis structure
	TxIndexer        txindex.TxIndexer
	ConsensusReactor *consensus.Reactor
	EventBus         *types.EventBus // thread safe
	Mempool          mempl.Mempool

	Logger log.Logger

	Config cfg.RPCConfig
}

//----------------------------------------------

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

// latestHeight can be either latest committed or uncommitted (+1) height.
func getHeight(latestHeight int64, heightPtr *int64) (int64, error) {
	if heightPtr != nil {
		height := *heightPtr
		if height <= 0 {
			return 0, fmt.Errorf("height must be greater than 0, but got %d", height)
		}
		if height > latestHeight {
			return 0, fmt.Errorf("height %d must be less than or equal to the current blockchain height %d",
				height, latestHeight)
		}
		base := env.BlockStore.Base()
		if height < base {
			return 0, fmt.Errorf("height %v is not available, blocks pruned at height %v",
				height, base)
		}
		return height, nil
	}
	return latestHeight, nil
}

func latestUncommittedHeight() int64 {
	nodeIsSyncing := env.ConsensusReactor.FastSync()
	if nodeIsSyncing {
		return env.BlockStore.Height()
	}
	return env.BlockStore.Height() + 1
}
