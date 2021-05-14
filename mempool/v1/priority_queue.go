package v1

import (
	"container/heap"

	tmsync "github.com/tendermint/tendermint/libs/sync"
)

var _ heap.Interface = (*TxPriorityQueue)(nil)

// TxPriorityQueue defines a thread-safe priority queue for valid transactions.
type TxPriorityQueue struct {
	mtx tmsync.RWMutex
	txs []*WrappedTx
}

func NewTxPriorityQueue() *TxPriorityQueue {
	pq := &TxPriorityQueue{
		txs: make([]*WrappedTx, 0),
	}

	heap.Init(pq)

	return pq
}

// NumTxs returns the number of transactions in the priority queue. It is
// thread safe.
func (pq *TxPriorityQueue) NumTxs() int {
	pq.mtx.RLock()
	defer pq.mtx.RUnlock()

	return len(pq.txs)
}

// PushTx adds a valid transaction to the priority queue. It is thread safe.
func (pq *TxPriorityQueue) PushTx(tx *WrappedTx) {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()

	heap.Push(pq, tx)
}

// PopTx removes the top priority transaction from the queue. It is thread safe.
func (pq *TxPriorityQueue) PopTx() *WrappedTx {
	pq.mtx.Lock()
	defer pq.mtx.Unlock()

	x := heap.Pop(pq)
	if x != nil {
		return x.(*WrappedTx)
	}

	return nil
}

// Push implements the Heap interface.
//
// NOTE: A caller should never call this. Use PushTx instead.
func (pq *TxPriorityQueue) Push(x interface{}) {
	n := len(pq.txs)
	item := x.(*WrappedTx)
	item.heapIndex = n
	pq.txs = append(pq.txs, item)
}

// Pop implements the Heap interface.
//
// NOTE: A caller should never call this. Use PopTx instead.
func (pq *TxPriorityQueue) Pop() interface{} {
	old := pq.txs
	n := len(old)
	item := old[n-1]
	old[n-1] = nil      // avoid memory leak
	item.heapIndex = -1 // for safety
	pq.txs = old[0 : n-1]
	return item
}

// Len implements the Heap interface.
//
// NOTE: A caller should never call Len. Use NumTxs instead.
func (pq *TxPriorityQueue) Len() int {
	return len(pq.txs)
}

// Less implements the Heap interface. It returns true if the transaction at
// position i in the queue is of less priority than the transaction at position j.
func (pq *TxPriorityQueue) Less(i, j int) bool {
	if pq.txs[i].Priority == pq.txs[j].Priority {
		return pq.txs[i].Timestamp.Unix() < pq.txs[j].Timestamp.Unix()
	}

	// We want Pop to give us the highest, not lowest, priority so we use greater
	// than here.
	return pq.txs[i].Priority > pq.txs[j].Priority
}

// Swap implements the Heap interface. It swaps two transactions in the queue.
func (pq *TxPriorityQueue) Swap(i, j int) {
	pq.txs[i], pq.txs[j] = pq.txs[j], pq.txs[i]
	pq.txs[i].heapIndex = i
	pq.txs[j].heapIndex = j
}
