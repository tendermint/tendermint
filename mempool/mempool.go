package mempool

import (
	"bytes"
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	abci "github.com/tendermint/abci/types"
	auto "github.com/tendermint/go-autofile"
	"github.com/tendermint/go-clist"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
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

TODO: Better handle abci client errors. (make it automatically handle connection errors)

*/

const cacheSize = 100000

type Mempool struct {
	config cfg.Config

	proxyMtx      sync.Mutex
	proxyAppConn  proxy.AppConnMempool
	txs           *clist.CList    // concurrent linked-list of good txs
	counter       int64           // simple incrementing counter
	height        int             // the last block Update()'d to
	rechecking    int32           // for re-checking filtered txs on Update()
	recheckCursor *clist.CElement // next expected response
	recheckEnd    *clist.CElement // re-checking stops here

	// Keep a cache of already-seen txs.
	// This reduces the pressure on the proxyApp.
	cache *txCache

	// A log of mempool txs
	wal *auto.AutoFile
}

func NewMempool(config cfg.Config, proxyAppConn proxy.AppConnMempool) *Mempool {
	mempool := &Mempool{
		config:        config,
		proxyAppConn:  proxyAppConn,
		txs:           clist.New(),
		counter:       0,
		height:        0,
		rechecking:    0,
		recheckCursor: nil,
		recheckEnd:    nil,

		cache: newTxCache(cacheSize),
	}
	mempool.initWAL()
	proxyAppConn.SetResponseCallback(mempool.resCb)
	return mempool
}

func (mem *Mempool) initWAL() {
	walDir := mem.config.GetString("mempool_wal_dir")
	if walDir != "" {
		err := EnsureDir(walDir, 0700)
		if err != nil {
			log.Error("Error ensuring Mempool wal dir", "error", err)
			PanicSanity(err)
		}
		af, err := auto.OpenAutoFile(walDir + "/wal")
		if err != nil {
			log.Error("Error opening Mempool wal file", "error", err)
			PanicSanity(err)
		}
		mem.wal = af
	}
}

// consensus must be able to hold lock to safely update
func (mem *Mempool) Lock() {
	mem.proxyMtx.Lock()
}

func (mem *Mempool) Unlock() {
	mem.proxyMtx.Unlock()
}

// Number of transactions in the mempool clist
func (mem *Mempool) Size() int {
	return mem.txs.Len()
}

// Remove all transactions from mempool and cache
func (mem *Mempool) Flush() {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	mem.cache.Reset()

	for e := mem.txs.Front(); e != nil; e = e.Next() {
		mem.txs.Remove(e)
		e.DetachPrev()
	}
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
func (mem *Mempool) CheckTx(tx types.Tx, cb func(*abci.Response)) (err error) {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	// CACHE
	if mem.cache.Exists(tx) {
		if cb != nil {
			cb(&abci.Response{
				Value: &abci.Response_CheckTx{
					&abci.ResponseCheckTx{
						Code: abci.CodeType_BadNonce, // TODO or duplicate tx
						Log:  "Duplicate transaction (ignored)",
					},
				},
			})
		}
		return nil
	}
	mem.cache.Push(tx)
	// END CACHE

	// WAL
	if mem.wal != nil {
		// TODO: Notify administrators when WAL fails
		mem.wal.Write([]byte(tx))
		mem.wal.Write([]byte("\n"))
	}
	// END WAL

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

// ABCI callback function
func (mem *Mempool) resCb(req *abci.Request, res *abci.Response) {
	if mem.recheckCursor == nil {
		mem.resCbNormal(req, res)
	} else {
		mem.resCbRecheck(req, res)
	}
}

func (mem *Mempool) resCbNormal(req *abci.Request, res *abci.Response) {
	switch r := res.Value.(type) {
	case *abci.Response_CheckTx:
		if r.CheckTx.Code == abci.CodeType_OK {
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
			mem.cache.Remove(req.GetCheckTx().Tx)

			// TODO: handle other retcodes
		}
	default:
		// ignore other messages
	}
}

func (mem *Mempool) resCbRecheck(req *abci.Request, res *abci.Response) {
	switch r := res.Value.(type) {
	case *abci.Response_CheckTx:
		memTx := mem.recheckCursor.Value.(*mempoolTx)
		if !bytes.Equal(req.GetCheckTx().Tx, memTx.tx) {
			PanicSanity(Fmt("Unexpected tx response from proxy during recheck\n"+
				"Expected %X, got %X", r.CheckTx.Data, memTx.tx))
		}
		if r.CheckTx.Code == abci.CodeType_OK {
			// Good, nothing to do.
		} else {
			// Tx became invalidated due to newly committed block.
			mem.txs.Remove(mem.recheckCursor)
			mem.recheckCursor.DetachPrev()

			// remove from cache (it might be good later)
			mem.cache.Remove(req.GetCheckTx().Tx)
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
// If maxTxs is -1, there is no cap on returned transactions.
func (mem *Mempool) Reap(maxTxs int) types.Txs {
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
func (mem *Mempool) collectTxs(maxTxs int) types.Txs {
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
func (mem *Mempool) Update(height int, txs types.Txs) {
	// TODO: check err ?
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
		// Remove the tx if it's alredy in a block.
		if _, ok := blockTxsMap[string(memTx.tx)]; ok {
			// remove from clist
			mem.txs.Remove(e)
			e.DetachPrev()

			// NOTE: we don't remove committed txs from the cache.
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

//--------------------------------------------------------------------------------

type txCache struct {
	mtx  sync.Mutex
	size int
	map_ map[string]struct{}
	list *list.List // to remove oldest tx when cache gets too big
}

func newTxCache(cacheSize int) *txCache {
	return &txCache{
		size: cacheSize,
		map_: make(map[string]struct{}, cacheSize),
		list: list.New(),
	}
}

func (cache *txCache) Reset() {
	cache.mtx.Lock()
	cache.map_ = make(map[string]struct{}, cacheSize)
	cache.list.Init()
	cache.mtx.Unlock()
}

func (cache *txCache) Exists(tx types.Tx) bool {
	cache.mtx.Lock()
	_, exists := cache.map_[string(tx)]
	cache.mtx.Unlock()
	return exists
}

// Returns false if tx is in cache.
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
		// but deleting a non-existant element is fine
		delete(cache.map_, string(poppedTx))
		cache.list.Remove(popped)
	}
	cache.map_[string(tx)] = struct{}{}
	cache.list.PushBack(tx)
	return true
}

func (cache *txCache) Remove(tx types.Tx) {
	cache.mtx.Lock()
	delete(cache.map_, string(tx))
	cache.mtx.Unlock()
}
