package mempool

import (
	"bytes"
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/go-clist"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
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

TODO: Better handle tmsp client errors. (make it automatically handle connection errors)

*/

const cacheSize = 100000

type Mempool struct {
	config cfg.Config

	proxyMtx      sync.Mutex
	proxyAppConn  proxy.AppConn
	txs           *clist.CList    // concurrent linked-list of good txs
	counter       int64           // simple incrementing counter
	height        int             // the last block Update()'d to
	rechecking    int32           // for re-checking filtered txs on Update()
	recheckCursor *clist.CElement // next expected response
	recheckEnd    *clist.CElement // re-checking stops here

	// Keep a cache of already-seen txs.
	// This reduces the pressure on the proxyApp.
	cacheMap  map[string]struct{}
	cacheList *list.List
}

func NewMempool(config cfg.Config, proxyAppConn proxy.AppConn) *Mempool {
	mempool := &Mempool{
		config:        config,
		proxyAppConn:  proxyAppConn,
		txs:           clist.New(),
		counter:       0,
		height:        0,
		rechecking:    0,
		recheckCursor: nil,
		recheckEnd:    nil,

		cacheMap:  make(map[string]struct{}, cacheSize),
		cacheList: list.New(),
	}
	proxyAppConn.SetResponseCallback(mempool.resCb)
	return mempool
}

func (mem *Mempool) Lock() {
	mem.proxyMtx.Lock()
}

func (mem *Mempool) Unlock() {
	mem.proxyMtx.Unlock()
}

func (mem *Mempool) Size() int {
	return mem.txs.Len()
}

// Return the first element of mem.txs for peer goroutines to call .NextWait() on.
// Blocks until txs has elements.
func (mem *Mempool) TxsFrontWait() *clist.CElement {
	return mem.txs.FrontWait()
}

// Try a new transaction in the mempool.
// Potentially blocking if we're blocking on Update() or Reap().
// cb: A callback from the CheckTx command.
//     It gets called from another goroutine.
// CONTRACT: Either cb will get called, or err returned.
func (mem *Mempool) CheckTx(tx types.Tx, cb func(*tmsp.Response)) (err error) {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	// CACHE
	if _, exists := mem.cacheMap[string(tx)]; exists {
		if cb != nil {
			cb(&tmsp.Response{
				Value: &tmsp.Response_CheckTx{
					&tmsp.ResponseCheckTx{
						Code: tmsp.CodeType_BadNonce, // TODO or duplicate tx
						Log:  "Duplicate transaction (ignored)",
					},
				},
			})
		}
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

	// NOTE: proxyAppConn may error if tx buffer is full
	if err = mem.proxyAppConn.Error(); err != nil {
		return err
	}
	reqRes := mem.proxyAppConn.CheckTxAsync(tx)
	if cb != nil {
		reqRes.SetCallback(cb)
	}

	return nil
}

func (mem *Mempool) removeTxFromCacheMap(tx []byte) {
	mem.proxyMtx.Lock()
	delete(mem.cacheMap, string(tx))
	mem.proxyMtx.Unlock()

}

// TMSP callback function
func (mem *Mempool) resCb(req *tmsp.Request, res *tmsp.Response) {
	if mem.recheckCursor == nil {
		mem.resCbNormal(req, res)
	} else {
		mem.resCbRecheck(req, res)
	}
}

func (mem *Mempool) resCbNormal(req *tmsp.Request, res *tmsp.Response) {
	switch r := res.Value.(type) {
	case *tmsp.Response_CheckTx:
		if r.CheckTx.Code == tmsp.CodeType_OK {
			mem.counter++
			memTx := &mempoolTx{
				counter: mem.counter,
				height:  int64(mem.height),
				tx:      req.GetCheckTx().Tx,
			}
			mem.txs.PushBack(memTx)
		} else {
			// ignore bad transaction
			log.Info("Bad Transaction", "res", r)

			// remove from cache (it might be good later)
			mem.removeTxFromCacheMap(req.GetCheckTx().Tx)

			// TODO: handle other retcodes
		}
	default:
		// ignore other messages
	}
}

func (mem *Mempool) resCbRecheck(req *tmsp.Request, res *tmsp.Response) {
	switch r := res.Value.(type) {
	case *tmsp.Response_CheckTx:
		memTx := mem.recheckCursor.Value.(*mempoolTx)
		if !bytes.Equal(req.GetCheckTx().Tx, memTx.tx) {
			PanicSanity(Fmt("Unexpected tx response from proxy during recheck\n"+
				"Expected %X, got %X", r.CheckTx.Data, memTx.tx))
		}
		if r.CheckTx.Code == tmsp.CodeType_OK {
			// Good, nothing to do.
		} else {
			// Tx became invalidated due to newly committed block.
			mem.txs.Remove(mem.recheckCursor)
			mem.recheckCursor.DetachPrev()

			// remove from cache (it might be good later)
			mem.removeTxFromCacheMap(req.GetCheckTx().Tx)
		}
		if mem.recheckCursor == mem.recheckEnd {
			mem.recheckCursor = nil
		} else {
			mem.recheckCursor = mem.recheckCursor.Next()
		}
		if mem.recheckCursor == nil {
			// Done!
			atomic.StoreInt32(&mem.rechecking, 0)
			log.Info("Done rechecking txs")
		}
	default:
		// ignore other messages
	}
}

// Get the valid transactions remaining
// If maxTxs is 0, there is no cap.
func (mem *Mempool) Reap(maxTxs int) []types.Tx {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	for atomic.LoadInt32(&mem.rechecking) > 0 {
		// TODO: Something better?
		time.Sleep(time.Millisecond * 10)
	}

	txs := mem.collectTxs(maxTxs)
	return txs
}

// maxTxs: -1 means uncapped, 0 means none
func (mem *Mempool) collectTxs(maxTxs int) []types.Tx {
	if maxTxs == 0 {
		return []types.Tx{}
	} else if maxTxs < 0 {
		maxTxs = mem.txs.Len()
	}
	txs := make([]types.Tx, 0, MinInt(mem.txs.Len(), maxTxs))
	for e := mem.txs.Front(); e != nil && len(txs) < maxTxs; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		txs = append(txs, memTx.tx)
	}
	return txs
}

// Tell mempool that these txs were committed.
// Mempool will discard these txs.
// NOTE: this should be called *after* block is committed by consensus.
// NOTE: unsafe; Lock/Unlock must be managed by caller
func (mem *Mempool) Update(height int, txs []types.Tx) {
	//	mem.proxyMtx.Lock()
	//	defer mem.proxyMtx.Unlock()
	mem.proxyAppConn.FlushSync() // To flush async resCb calls e.g. from CheckTx

	// First, create a lookup map of txns in new txs.
	txsMap := make(map[string]struct{})
	for _, tx := range txs {
		txsMap[string(tx)] = struct{}{}
	}

	// Set height
	mem.height = height
	// Remove transactions that are already in txs.
	goodTxs := mem.filterTxs(txsMap)
	// Recheck mempool txs if any txs were committed in the block
	// NOTE/XXX: in some apps a tx could be invalidated due to EndBlock,
	//	so we really still do need to recheck, but this is for debugging
	if mem.config.GetBool("mempool_recheck") &&
		(mem.config.GetBool("mempool_recheck_empty") || len(txs) > 0) {
		log.Info("Recheck txs", "numtxs", len(goodTxs))
		mem.recheckTxs(goodTxs)
		// At this point, mem.txs are being rechecked.
		// mem.recheckCursor re-scans mem.txs and possibly removes some txs.
		// Before mem.Reap(), we should wait for mem.recheckCursor to be nil.
	}
}

func (mem *Mempool) filterTxs(blockTxsMap map[string]struct{}) []types.Tx {
	goodTxs := make([]types.Tx, 0, mem.txs.Len())
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		// Remove the tx if its alredy in a block.
		if _, ok := blockTxsMap[string(memTx.tx)]; ok {
			// remove from clist
			mem.txs.Remove(e)
			e.DetachPrev()

			// remove from mempool cache
			// we only enforce "at-least once" semantics and
			// leave it to the application to implement "only-once"
			// via eg. sequence numbers, utxos, etc.
			// NOTE: expects caller of filterTxs to hold the lock
			// (so we can't use mem.removeTxFromCacheMap)
			delete(mem.cacheMap, string(memTx.tx))
			continue
		}
		// Good tx!
		goodTxs = append(goodTxs, memTx.tx)
	}
	return goodTxs
}

// NOTE: pass in goodTxs because mem.txs can mutate concurrently.
func (mem *Mempool) recheckTxs(goodTxs []types.Tx) {
	if len(goodTxs) == 0 {
		return
	}
	atomic.StoreInt32(&mem.rechecking, 1)
	mem.recheckCursor = mem.txs.Front()
	mem.recheckEnd = mem.txs.Back()

	// Push txs to proxyAppConn
	// NOTE: resCb() may be called concurrently.
	for _, tx := range goodTxs {
		mem.proxyAppConn.CheckTxAsync(tx)
	}
	mem.proxyAppConn.FlushAsync()
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
