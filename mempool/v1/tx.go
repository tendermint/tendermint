package v1

import (
	"time"

	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

// WrappedTx defines a wrapper around a raw transaction with additional metadata
// that is used for indexing.
type WrappedTx struct {
	// Tx represents the raw binary transaction data.
	Tx types.Tx

	// Priority defines the transaction's priority as specified by the application
	// in the ResponseCheckTx response.
	Priority int64

	// Sender defines the transaction's sender as specified by the application in
	// the ResponseCheckTx response.
	Sender string

	// Timestamp is the time at which the node first received the transaction from
	// a peer. It is used as a second dimension is prioritizing transactions when
	// two transactions have the same priority.
	Timestamp time.Time

	// heapIndex defines the index of the item in the heap
	heapIndex int

	// peers records a mapping of all peers that sent a given transaction
	peers map[uint16]struct{}
}

func (wtx *WrappedTx) Size() int {
	return len(wtx.Tx)
}

// TxStore implements a thread-safe mapping of valid transaction(s).
type TxStore struct {
	mtx       tmsync.RWMutex
	senderTxs map[string]*WrappedTx // sender is defined by the ABCI application
	hashTxs   map[[mempool.TxKeySize]byte]*WrappedTx
}

func NewTxStore() *TxStore {
	return &TxStore{
		senderTxs: make(map[string]*WrappedTx),
		hashTxs:   make(map[[mempool.TxKeySize]byte]*WrappedTx),
	}
}

func (txs *TxStore) GetTxBySender(sender string) *WrappedTx {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	return txs.senderTxs[sender]
}

func (txs *TxStore) GetTxByHash(hash [mempool.TxKeySize]byte) *WrappedTx {
	txs.mtx.RLock()
	defer txs.mtx.RUnlock()

	return txs.hashTxs[hash]
}

func (txs *TxStore) SetTx(wtx *WrappedTx) {
	txs.mtx.Lock()
	defer txs.mtx.Unlock()

	if len(wtx.Sender) > 0 {
		txs.senderTxs[wtx.Sender] = wtx
	}

	txs.hashTxs[mempool.TxKey(wtx.Tx)] = wtx
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
