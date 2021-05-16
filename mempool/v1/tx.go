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

// TxMap implements a thread-safe mapping of valid transaction(s).
type TxMap struct {
	mtx       tmsync.RWMutex
	senderTxs map[string]*WrappedTx // sender is defined by the ABCI application
	hashTxs   map[[mempool.TxKeySize]byte]*WrappedTx
}

func NewTxMap() *TxMap {
	return &TxMap{
		senderTxs: make(map[string]*WrappedTx),
		hashTxs:   make(map[[mempool.TxKeySize]byte]*WrappedTx),
	}
}

func (txm *TxMap) GetTxBySender(sender string) *WrappedTx {
	txm.mtx.RLock()
	defer txm.mtx.RUnlock()

	return txm.senderTxs[sender]
}

func (txm *TxMap) GetTxByHash(hash [mempool.TxKeySize]byte) *WrappedTx {
	txm.mtx.RLock()
	defer txm.mtx.RUnlock()

	return txm.hashTxs[hash]
}

func (txm *TxMap) SetTx(wtx *WrappedTx) {
	txm.mtx.Lock()
	defer txm.mtx.Unlock()

	txm.senderTxs[wtx.Sender] = wtx
	txm.hashTxs[mempool.TxKey(wtx.Tx)] = wtx
}

// GetOrSetPeerByTxHash looks up a WrappedTx by transaction hash and adds the
// given peerID to the WrappedTx's set of peers that sent us this transaction.
// We return true if we've already recorded the given peer for this transaction
// and false otherwise. If the transaction does not exist by hash, we return
// (nil, false).
func (txm *TxMap) GetOrSetPeerByTxHash(hash [mempool.TxKeySize]byte, peerID uint16) (*WrappedTx, bool) {
	txm.mtx.Lock()
	defer txm.mtx.Unlock()

	wtx := txm.hashTxs[hash]
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
