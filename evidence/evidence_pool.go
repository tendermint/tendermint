package evpool

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

const cacheSize = 100000

// EvidencePool maintains a set of valid uncommitted evidence.
type EvidencePool struct {
	config *cfg.EvidencePoolConfig

	mtx      sync.Mutex
	height   int // the last block Update()'d to
	evidence types.Evidences
	// TODO: evidenceCache

	// TODO: need to persist evidence so we never lose it

	logger log.Logger
}

func NewEvidencePool(config *cfg.EvidencePoolConfig, height int) *EvidencePool {
	evpool := &EvidencePool{
		config: config,
		height: height,
		logger: log.NewNopLogger(),
	}
	evpool.initWAL()
	return evpool
}

// SetLogger sets the Logger.
func (evpool *EvidencePool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// Evidence returns a copy of the pool's evidence.
func (evpool *EvidencePool) Evidence() types.Evidences {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()

	evCopy := make(types.Evidences, len(evpool.evidence))
	for i, ev := range evpool.evidence {
		evCopy[i] = ev
	}
	return evCopy
}

// Size returns the number of pieces of evidence in the evpool.
func (evpool *EvidencePool) Size() int {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	return len(evpool.evidence)
}

// Flush removes all evidence from the evpool
func (evpool *EvidencePool) Flush() {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	evpool.evidence = make(types.Evidence)
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *EvidencePool) AddEvidence(evidence types.Evidence) (err error) {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()

	if evpool.evidence.Has(evidence) {
		return fmt.Errorf("Evidence already exists", "evidence", evidence)
	}
	cs.Logger.Info("Found conflicting vote. Recording evidence", "evidence", ev)
	evpool.evidence = append(evpool.evidence, ev)
	// TODO: write to disk ? WAL ?
	return nil
}

// Update informs the evpool that the given evidence was committed and can be discarded.
// NOTE: this should be called *after* block is committed by consensus.
func (evpool *EvidencePool) Update(height int, evidence types.Evidences) {

	// First, create a lookup map of txns in new txs.
	evMap := make(map[string]struct{})
	for _, ev := range evidence {
		evMap[string(evidence.Hash())] = struct{}{}
	}

	// Set height
	evpool.height = height

	// Remove evidence that is already committed .
	goodEvidence := evpool.filterEvidence(evMap)
	_ = goodEvidence

}

// TODO:
func (evpool *EvidencePool) filterTxs(blockTxsMap map[string]struct{}) []types.Tx {
	goodTxs := make([]types.Tx, 0, evpool.txs.Len())
	for e := evpool.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*evpoolTx)
		// Remove the tx if it's alredy in a block.
		if _, ok := blockTxsMap[string(memTx.tx)]; ok {
			// remove from clist
			evpool.txs.Remove(e)
			e.DetachPrev()

			// NOTE: we don't remove committed txs from the cache.
			continue
		}
		// Good tx!
		goodTxs = append(goodTxs, memTx.tx)
	}
	return goodTxs
}

//--------------------------------------------------------------------------------

// evpoolTx is a transaction that successfully ran
type evpoolEvidence struct {
	counter  int64          // a simple incrementing counter
	height   int64          // height that this tx had been validated in
	evidence types.Evidence //
}

// Height returns the height for this transaction
func (memTx *evpoolTx) Height() int {
	return int(atomic.LoadInt64(&memTx.height))
}

//--------------------------------------------------------------------------------
// TODO:

// txCache maintains a cache of evidence
type txCache struct {
	mtx  sync.Mutex
	size int
	map_ map[string]struct{}
	list *list.List // to remove oldest tx when cache gets too big
}

// newTxCache returns a new txCache.
func newTxCache(cacheSize int) *txCache {
	return &txCache{
		size: cacheSize,
		map_: make(map[string]struct{}, cacheSize),
		list: list.New(),
	}
}

// Reset resets the txCache to empty.
func (cache *txCache) Reset() {
	cache.mtx.Lock()
	cache.map_ = make(map[string]struct{}, cacheSize)
	cache.list.Init()
	cache.mtx.Unlock()
}

// Exists returns true if the given tx is cached.
func (cache *txCache) Exists(tx types.Tx) bool {
	cache.mtx.Lock()
	_, exists := cache.map_[string(tx)]
	cache.mtx.Unlock()
	return exists
}

// Push adds the given tx to the txCache. It returns false if tx is already in the cache.
func (cache *txCache) Push(tx types.Tx) bool {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	if _, exists := cache.map_[string(tx)]; exists {
		return false
	}

	if cache.list.Len() >= cache.size {
		popped := cache.list.Front()
		poppedTx := popped.Value.(types.Tx)
		// NOTE: the tx may have already been removed from the map
		// but deleting a non-existent element is fine
		delete(cache.map_, string(poppedTx))
		cache.list.Remove(popped)
	}
	cache.map_[string(tx)] = struct{}{}
	cache.list.PushBack(tx)
	return true
}

// Remove removes the given tx from the cache.
func (cache *txCache) Remove(tx types.Tx) {
	cache.mtx.Lock()
	delete(cache.map_, string(tx))
	cache.mtx.Unlock()
}
