package mempool

import (
	"bytes"
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

The mempool pushes new txs onto the proxyAppCtx.
It gets a stream of (req, res) tuples from the proxy.
The memool stores good txs in a concurrent linked-list.

Multiple concurrent go-routines can traverse this linked-list
safely by calling .NextWait() on each element.

So we have several go-routines:
1. Consensus calling Update() and Reap() synchronously
2. Many mempool reactor's peer routines calling AppendTx()
3. Many mempool reactor's peer routines traversing the txs linked list
4. Another goroutine calling GarbageCollectTxs() periodically

To manage these goroutines, there are three methods of locking.
1. Mutations to the linked-list is protected by an internal mtx (CList is goroutine-safe)
2. Mutations to the linked-list elements are atomic
3. AppendTx() calls can be paused upon Update() and Reap(), protected by .proxyMtx

Garbage collection of old elements from mempool.txs is handlde via
the DetachPrev() call, which makes old elements not reachable by
peer broadcastTxRoutine() automatically garbage collected.

*/

const cacheSize = 100000

type Mempool struct {
	proxyMtx    sync.Mutex
	proxyAppCtx proxy.AppContext
	txs         *clist.CList    // concurrent linked-list of good txs
	counter     int64           // simple incrementing counter
	height      int             // the last block Update()'d to
	expected    *clist.CElement // pointer to .txs for next response

	// Keep a cache of already-seen txs.
	// This reduces the pressure on the proxyApp.
	cacheMap  map[string]struct{}
	cacheList *list.List
}

func NewMempool(proxyAppCtx proxy.AppContext) *Mempool {
	mempool := &Mempool{
		proxyAppCtx: proxyAppCtx,
		txs:         clist.New(),
		counter:     0,
		height:      0,
		expected:    nil,

		cacheMap:  make(map[string]struct{}, cacheSize),
		cacheList: list.New(),
	}
	proxyAppCtx.SetResponseCallback(mempool.resCb)
	return mempool
}

// Return the first element of mem.txs for peer goroutines to call .NextWait() on.
// Blocks until txs has elements.
func (mem *Mempool) TxsFrontWait() *clist.CElement {
	return mem.txs.FrontWait()
}

// Try a new transaction in the mempool.
// Potentially blocking if we're blocking on Update() or Reap().
func (mem *Mempool) AppendTx(tx types.Tx) (err error) {
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

	if err = mem.proxyAppCtx.Error(); err != nil {
		return err
	}
	mem.proxyAppCtx.AppendTxAsync(tx)
	return nil
}

// TMSP callback function
// CONTRACT: No other goroutines mutate mem.expected concurrently.
func (mem *Mempool) resCb(req tmsp.Request, res tmsp.Response) {
	switch res := res.(type) {
	case tmsp.ResponseAppendTx:
		reqAppendTx := req.(tmsp.RequestAppendTx)
		if mem.expected == nil { // Normal operation
			if res.RetCode == tmsp.RetCodeOK {
				mem.counter++
				memTx := &mempoolTx{
					counter: mem.counter,
					height:  int64(mem.height),
					tx:      reqAppendTx.TxBytes,
				}
				mem.txs.PushBack(memTx)
			} else {
				// ignore bad transaction
				// TODO: handle other retcodes
			}
		} else { // During Update()
			// TODO Log sane warning if mem.expected is nil.
			memTx := mem.expected.Value.(*mempoolTx)
			if !bytes.Equal(reqAppendTx.TxBytes, memTx.tx) {
				PanicSanity("Unexpected tx response from proxy")
			}
			if res.RetCode == tmsp.RetCodeOK {
				// Good, nothing to do.
			} else {
				// TODO: handle other retcodes
				// Tx became invalidated due to newly committed block.
				// NOTE: Concurrent traversal of mem.txs via CElement.Next() still works.
				mem.txs.Remove(mem.expected)
				mem.expected.DetachPrev()
			}
			mem.expected = mem.expected.Next()
		}
	default:
		// ignore other messages
	}
}

// Get the valid transactions run so far, and the hash of
// the application state that results from those transactions.
func (mem *Mempool) Reap() ([]types.Tx, []byte, error) {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	// First, get the hash of txs run so far
	hash, err := mem.proxyAppCtx.GetHashSync()
	if err != nil {
		return nil, nil, err
	}

	// And collect all the transactions.
	txs := mem.collectTxs()

	return txs, hash, nil
}

func (mem *Mempool) collectTxs() []types.Tx {
	txs := make([]types.Tx, 0, mem.txs.Len())
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		txs = append(txs, memTx.tx)
	}
	return txs
}

// "block" is the new block that was committed.
// Txs that are present in "block" are discarded from mempool.
// NOTE: this should be called *after* block is committed by consensus.
// CONTRACT: block is valid and next in sequence.
func (mem *Mempool) Update(block *types.Block) error {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	// Rollback mempool synchronously
	// TODO: test that proxyAppCtx's state matches the block's
	err := mem.proxyAppCtx.RollbackSync()
	if err != nil {
		return err
	}

	// First, create a lookup map of txns in new block.
	blockTxsMap := make(map[string]struct{})
	for _, tx := range block.Data.Txs {
		blockTxsMap[string(tx)] = struct{}{}
	}

	// Remove transactions that are already in block.
	// Return the remaining potentially good txs.
	goodTxs := mem.filterTxs(block.Height, blockTxsMap)

	// Set height and expected
	mem.height = block.Height
	mem.expected = mem.txs.Front()

	// Push good txs to proxyAppCtx
	// NOTE: resCb() may be called concurrently.
	for _, tx := range goodTxs {
		mem.proxyAppCtx.AppendTxAsync(tx)
		if err := mem.proxyAppCtx.Error(); err != nil {
			return err
		}
	}

	// NOTE: Even though we return immediately without e.g.
	// calling mem.proxyAppCtx.FlushSync(),
	// New mempool txs will still have to wait until
	// all goodTxs are re-processed.
	// So we could make synchronous calls here to proxyAppCtx.

	return nil
}

func (mem *Mempool) filterTxs(height int, blockTxsMap map[string]struct{}) []types.Tx {
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
		atomic.StoreInt64(&memTx.height, int64(height))
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
