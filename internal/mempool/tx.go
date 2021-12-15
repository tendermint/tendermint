package mempool

import (
	"sort"
	"sync"
	"time"

	"github.com/tendermint/tendermint/internal/libs/clist"
	"github.com/tendermint/tendermint/types"
)

// TxInfo are parameters that get passed when attempting to add a tx to the
// mempool.
type TxInfo struct {
	// SenderID is the internal peer ID used in the mempool to identify the
	// sender, storing two bytes with each transaction instead of 20 bytes for
	// the types.NodeID.
	SenderID uint16

	// SenderNodeID is the actual types.NodeID of the sender.
	SenderNodeID types.NodeID
}

// WrappedTx defines a wrapper around a raw transaction with additional metadata
// that is used for indexing.
type WrappedTx struct {
	// tx represents the raw binary transaction data
	tx types.Tx

	// hash defines the transaction hash and the primary key used in the mempool
	hash types.TxKey

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
	mtx       sync.RWMutex
	hashTxs   map[types.TxKey]*WrappedTx // primary index
	senderTxs map[string]*WrappedTx      // sender is defined by the ABCI application
}

func NewTxStore() *TxStore {
	return &TxStore{
		senderTxs: make(map[string]*WrappedTx),
		hashTxs:   make(map[types.TxKey]*WrappedTx),
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
func (txs *TxStore) GetTxByHash(hash types.TxKey) *WrappedTx {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	return txs.hashTxs[hash]
}

// IsTxRemoved returns true if a transaction by hash is marked as removed and
// false otherwise.
func (txs *TxStore) IsTxRemoved(hash types.TxKey) bool {
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

	txs.hashTxs[wtx.tx.Key()] = wtx
}

// RemoveTx removes a *WrappedTx from the transaction store. It deletes all
// indexes of the transaction.
func (txs *TxStore) RemoveTx(wtx *WrappedTx) {
	txs.mtx.Lock()
	defer txs.mtx.Unlock()

	if len(wtx.sender) > 0 {
		delete(txs.senderTxs, wtx.sender)
	}

	delete(txs.hashTxs, wtx.tx.Key())
	wtx.removed = true
}

// TxHasPeer returns true if a transaction by hash has a given peer ID and false
// otherwise. If the transaction does not exist, false is returned.
func (txs *TxStore) TxHasPeer(hash types.TxKey, peerID uint16) bool {
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
func (txs *TxStore) GetOrSetPeerByTxHash(hash types.TxKey, peerID uint16) (*WrappedTx, bool) {
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
// used to build generic transaction indexes in the mempool. It accepts a
// comparator function, less(a, b *WrappedTx) bool, that compares two WrappedTx
// references which is used during Insert in order to determine sorted order. If
// less returns true, a <= b.
type WrappedTxList struct {
	mtx  sync.RWMutex
	txs  []*WrappedTx
	less func(*WrappedTx, *WrappedTx) bool
}

func NewWrappedTxList(less func(*WrappedTx, *WrappedTx) bool) *WrappedTxList {
	return &WrappedTxList{
		txs:  make([]*WrappedTx, 0),
		less: less,
	}
}

// Size returns the number of WrappedTx objects in the list.
func (wtl *WrappedTxList) Size() int {
	wtl.mtx.RLock()
	defer wtl.mtx.RUnlock()

	return len(wtl.txs)
}

// Reset resets the list of transactions to an empty list.
func (wtl *WrappedTxList) Reset() {
	wtl.mtx.Lock()
	defer wtl.mtx.Unlock()

	wtl.txs = make([]*WrappedTx, 0)
}

// Insert inserts a WrappedTx reference into the sorted list based on the list's
// comparator function.
func (wtl *WrappedTxList) Insert(wtx *WrappedTx) {
	wtl.mtx.Lock()
	defer wtl.mtx.Unlock()

	i := sort.Search(len(wtl.txs), func(i int) bool {
		return wtl.less(wtl.txs[i], wtx)
	})

	if i == len(wtl.txs) {
		// insert at the end
		wtl.txs = append(wtl.txs, wtx)
		return
	}

	// Make space for the inserted element by shifting values at the insertion
	// index up one index.
	//
	// NOTE: The call to append does not allocate memory when cap(wtl.txs) > len(wtl.txs).
	wtl.txs = append(wtl.txs[:i+1], wtl.txs[i:]...)
	wtl.txs[i] = wtx
}

// Remove attempts to remove a WrappedTx from the sorted list.
func (wtl *WrappedTxList) Remove(wtx *WrappedTx) {
	wtl.mtx.Lock()
	defer wtl.mtx.Unlock()

	i := sort.Search(len(wtl.txs), func(i int) bool {
		return wtl.less(wtl.txs[i], wtx)
	})

	// Since the list is sorted, we evaluate all elements starting at i. Note, if
	// the element does not exist, we may potentially evaluate the entire remainder
	// of the list. However, a caller should not be expected to call Remove with a
	// non-existing element.
	for i < len(wtl.txs) {
		if wtl.txs[i] == wtx {
			wtl.txs = append(wtl.txs[:i], wtl.txs[i+1:]...)
			return
		}

		i++
	}
}
