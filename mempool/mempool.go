package mempool

import (
	"container/list"
	"sync"
	"sync/atomic"

	"github.com/tendermint/go-clist"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

/*

The mempool pushes new txs onto the proxyAppConn.
It gets a stream of (req, res) tuples from the proxy.
The memool stores good txs in a concurrent linked-list.

Multiple concurrent go-routines can traverse this linked-list
safely by calling .NextWait() on each element.

So we have several go-routines:
1. Consensus calling Update() and Reap() synchronously
2. Many mempool reactor's peer routines calling CheckTx()
3. Many mempool reactor's peer routines traversing the txs linked list
4. Another goroutine calling GarbageCollectTxs() periodically

To manage these goroutines, there are three methods of locking.
1. Mutations to the linked-list is protected by an internal mtx (CList is goroutine-safe)
2. Mutations to the linked-list elements are atomic
3. CheckTx() calls can be paused upon Update() and Reap(), protected by .proxyMtx

Garbage collection of old elements from mempool.txs is handlde via
the DetachPrev() call, which makes old elements not reachable by
peer broadcastTxRoutine() automatically garbage collected.

*/

const cacheSize = 100000

type Mempool struct {
	proxyMtx     sync.Mutex
	proxyAppConn proxy.AppConn
	txs          *clist.CList // concurrent linked-list of good txs
	counter      int64        // simple incrementing counter
	height       int          // the last block Update()'d to

	// Keep a cache of already-seen txs.
	// This reduces the pressure on the proxyApp.
	cacheMap  map[string]struct{}
	cacheList *list.List
}

func NewMempool(proxyAppConn proxy.AppConn) *Mempool {
	mempool := &Mempool{
		proxyAppConn: proxyAppConn,
		txs:          clist.New(),
		counter:      0,
		height:       0,

		cacheMap:  make(map[string]struct{}, cacheSize),
		cacheList: list.New(),
	}
	proxyAppConn.SetResponseCallback(mempool.resCb)
	return mempool
}

// Return the first element of mem.txs for peer goroutines to call .NextWait() on.
// Blocks until txs has elements.
func (mem *Mempool) TxsFrontWait() *clist.CElement {
	return mem.txs.FrontWait()
}

// Try a new transaction in the mempool.
// Potentially blocking if we're blocking on Update() or Reap().
func (mem *Mempool) CheckTx(tx types.Tx) (err error) {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	// CACHE
	if _, exists := mem.cacheMap[string(tx)]; exists {
		return nil
	}
	if mem.cacheList.Len() >= cacheSize {
		popped := mem.cacheList.Front()
		poppedTx := popped.Value.(types.Tx)
		delete(mem.cacheMap, string(poppedTx))
		mem.cacheList.Remove(popped)
	}
	mem.cacheMap[string(tx)] = struct{}{}
	mem.cacheList.PushBack(tx)
	// END CACHE

	if err = mem.proxyAppConn.Error(); err != nil {
		return err
	}
	mem.proxyAppConn.CheckTxAsync(tx)
	return nil
}

// TMSP callback function
func (mem *Mempool) resCb(req *tmsp.Request, res *tmsp.Response) {
	switch res.Type {
	case tmsp.MessageType_CheckTx:
		if tmsp.RetCode(res.Code) == tmsp.RetCodeOK {
			mem.counter++
			memTx := &mempoolTx{
				counter: mem.counter,
				height:  int64(mem.height),
				tx:      req.Data,
			}
			mem.txs.PushBack(memTx)
		} else {
			// ignore bad transaction
			// TODO: handle other retcodes
		}
	default:
		// ignore other messages
	}
}

// Get the valid transactions remaining
func (mem *Mempool) Reap() ([]types.Tx, error) {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	txs := mem.collectTxs()

	return txs, nil
}

func (mem *Mempool) collectTxs() []types.Tx {
	txs := make([]types.Tx, 0, mem.txs.Len())
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		txs = append(txs, memTx.tx)
	}
	return txs
}

// Tell mempool that these txs were committed.
// Mempool will discard these txs.
// NOTE: this should be called *after* block is committed by consensus.
func (mem *Mempool) Update(height int, txs []types.Tx) error {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	// First, create a lookup map of txns in new txs.
	txsMap := make(map[string]struct{})
	for _, tx := range txs {
		txsMap[string(tx)] = struct{}{}
	}

	// Set height
	mem.height = height

	// Remove transactions that are already in txs.
	mem.filterTxs(txsMap)

	return nil
}

func (mem *Mempool) filterTxs(blockTxsMap map[string]struct{}) []types.Tx {
	goodTxs := make([]types.Tx, 0, mem.txs.Len())
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		if _, ok := blockTxsMap[string(memTx.tx)]; ok {
			// Remove the tx since already in block.
			mem.txs.Remove(e)
			e.DetachPrev()
			continue
		}
		// Good tx!
		goodTxs = append(goodTxs, memTx.tx)
	}
	return goodTxs
}

//--------------------------------------------------------------------------------

// A transaction that successfully ran
type mempoolTx struct {
	counter int64    // a simple incrementing counter
	height  int64    // height that this tx had been validated in
	tx      types.Tx //
}

func (memTx *mempoolTx) Height() int {
	return int(atomic.LoadInt64(&memTx.height))
}
