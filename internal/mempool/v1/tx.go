package v1

import (
	"sort"
	"time"

	"github.com/tendermint/tendermint/internal/libs/clist"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/types"
)

// WrappedTx defines a wrapper around a raw transaction with additional metadata
// that is used for indexing.
type WrappedTx struct {
	// tx represents the raw binary transaction data
	tx types.Tx

	// hash defines the transaction hash and the primary key used in the mempool
	hash [mempool.TxKeySize]byte

	// height defines the height at which the transaction was validated at
	height int64

	// gasWanted defines the amount of gas the transaction sender requires
	gasWanted int64

	// priority defines the transaction's priority as specified by the application
	// in the ResponseCheckTx response.
	priority int64

	// sender defines the transaction's sender as specified by the application in
	// the ResponseCheckTx response.
	sender string

	// timestamp is the time at which the node first received the transaction from
	// a peer. It is used as a second dimension is prioritizing transactions when
	// two transactions have the same priority.
	timestamp time.Time

	// peers records a mapping of all peers that sent a given transaction
	peers map[uint16]struct{}

	// heapIndex defines the index of the item in the heap
	heapIndex int

	// gossipEl references the linked-list element in the gossip index
	gossipEl *clist.CElement

	// removed marks the transaction as removed from the mempool. This is set
	// during RemoveTx and is needed due to the fact that a given existing
	// transaction in the mempool can be evicted when it is simultaneously having
	// a reCheckTx callback executed.
	removed bool
}

func (wtx *WrappedTx) Size() int {
	return len(wtx.tx)
}

// TxStore implements a thread-safe mapping of valid transaction(s).
//
// NOTE:
// - Concurrent read-only access to a *WrappedTx object is OK. However, mutative
//   access is not allowed. Regardless, it is not expected for the mempool to
//   need mutative access.
type TxStore struct {
	mtx       tmsync.RWMutex
	hashTxs   map[[mempool.TxKeySize]byte]*WrappedTx // primary index
	senderTxs map[string]*WrappedTx                  // sender is defined by the ABCI application
}

func NewTxStore() *TxStore {
	return &TxStore{
		senderTxs: make(map[string]*WrappedTx),
		hashTxs:   make(map[[mempool.TxKeySize]byte]*WrappedTx),
	}
}

// Size returns the total number of transactions in the store.
func (txs *TxStore) Size() int {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	return len(txs.hashTxs)
}

// GetAllTxs returns all the transactions currently in the store.
func (txs *TxStore) GetAllTxs() []*WrappedTx {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	wTxs := make([]*WrappedTx, len(txs.hashTxs))
	i := 0
	for _, wtx := range txs.hashTxs {
		wTxs[i] = wtx
		i++
	}

	return wTxs
}

// GetTxBySender returns a *WrappedTx by the transaction's sender property
// defined by the ABCI application.
func (txs *TxStore) GetTxBySender(sender string) *WrappedTx {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	return txs.senderTxs[sender]
}

// GetTxByHash returns a *WrappedTx by the transaction's hash.
func (txs *TxStore) GetTxByHash(hash [mempool.TxKeySize]byte) *WrappedTx {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	return txs.hashTxs[hash]
}

// IsTxRemoved returns true if a transaction by hash is marked as removed and
// false otherwise.
func (txs *TxStore) IsTxRemoved(hash [mempool.TxKeySize]byte) bool {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	wtx, ok := txs.hashTxs[hash]
	if ok {
		return wtx.removed
	}

	return false
}

// SetTx stores a *WrappedTx by it's hash. If the transaction also contains a
// non-empty sender, we additionally store the transaction by the sender as
// defined by the ABCI application.
func (txs *TxStore) SetTx(wtx *WrappedTx) {
	txs.mtx.Lock()
	defer txs.mtx.Unlock()

	if len(wtx.sender) > 0 {
		txs.senderTxs[wtx.sender] = wtx
	}

	txs.hashTxs[mempool.TxKey(wtx.tx)] = wtx
}

// RemoveTx removes a *WrappedTx from the transaction store. It deletes all
// indexes of the transaction.
func (txs *TxStore) RemoveTx(wtx *WrappedTx) {
	txs.mtx.Lock()
	defer txs.mtx.Unlock()

	if len(wtx.sender) > 0 {
		delete(txs.senderTxs, wtx.sender)
	}

	delete(txs.hashTxs, mempool.TxKey(wtx.tx))
	wtx.removed = true
}

// TxHasPeer returns true if a transaction by hash has a given peer ID and false
// otherwise. If the transaction does not exist, false is returned.
func (txs *TxStore) TxHasPeer(hash [mempool.TxKeySize]byte, peerID uint16) bool {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	wtx := txs.hashTxs[hash]
	if wtx == nil {
		return false
	}

	_, ok := wtx.peers[peerID]
	return ok
}

// GetOrSetPeerByTxHash looks up a WrappedTx by transaction hash and adds the
// given peerID to the WrappedTx's set of peers that sent us this transaction.
// We return true if we've already recorded the given peer for this transaction
// and false otherwise. If the transaction does not exist by hash, we return
// (nil, false).
func (txs *TxStore) GetOrSetPeerByTxHash(hash [mempool.TxKeySize]byte, peerID uint16) (*WrappedTx, bool) {
	txs.mtx.Lock()
	defer txs.mtx.Unlock()

	wtx := txs.hashTxs[hash]
	if wtx == nil {
		return nil, false
	}

	if wtx.peers == nil {
		wtx.peers = make(map[uint16]struct{})
	}

	if _, ok := wtx.peers[peerID]; ok {
		return wtx, true
	}

	wtx.peers[peerID] = struct{}{}
	return wtx, false
}

// WrappedTxList implements a thread-safe list of *WrappedTx objects that can be
// used to build generic transaction indexes in the mempool.
type WrappedTxList struct {
	mtx tmsync.RWMutex
	txs []*WrappedTx
}

func NewWrappedTxList() *WrappedTxList {
	return &WrappedTxList{
		txs: make([]*WrappedTx, 0),
	}
}

// InsertAt inserts a reference to a WrappedTx object at index i in the the
// WrappedTxList keeping the list. A caller wishing to keep the list sorted must
// know the correct index, i, at which to insert. This can be done via a
// sort.Search with specific index searching criteria.
func (wtl *WrappedTxList) InsertAt(i int, wtx *WrappedTx) {
	wtl.mtx.Lock()
	defer wtl.mtx.Unlock()

	if i == len(wtl.txs) {
		// insert at the end
		wtl.txs = append(wtl.txs, wtx)
	}

	// Make space for the inserted element by shifting values at the insertion
	// index up one index.
	//
	// NOTE: The call to append does not allocate memory when cap(wtl.txs) > len(wtl.txs).
	wtl.txs = append(wtl.txs[:i+1], wtl.txs[i:]...)
	wtl.txs[i] = wtx
}

// GetSortedIndex searches for and returns the smallest index, via binary search,
// in the WrappedTxList where cmp(wtl[i]) returns true. This can be used to find
// the correct index to insert a new element at via InsertAt such that the list
// remains sorted given generic indexes.
func (wtl *WrappedTxList) GetSortedIndex(cmp func(*WrappedTx) bool) int {
	wtl.mtx.RLock()
	defer wtl.mtx.RUnlock()

	return sort.Search(len(wtl.txs), func(i int) bool {
		return cmp(wtl.txs[i])
	})
}
