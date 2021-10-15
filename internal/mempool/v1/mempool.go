package v1

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/libs/clist"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/types"
)

var _ mempool.Mempool = (*TxMempool)(nil)

// TxMempoolOption sets an optional parameter on the TxMempool.
type TxMempoolOption func(*TxMempool)

// TxMempool defines a prioritized mempool data structure used by the v1 mempool
// reactor. It keeps a thread-safe priority queue of transactions that is used
// when a block proposer constructs a block and a thread-safe linked-list that
// is used to gossip transactions to peers in a FIFO manner.
type TxMempool struct {
	logger       log.Logger
	metrics      *mempool.Metrics
	config       *config.MempoolConfig
	proxyAppConn proxy.AppConnMempool

	// txsAvailable fires once for each height when the mempool is not empty
	txsAvailable         chan struct{}
	notifiedTxsAvailable bool

	// height defines the last block height process during Update()
	height int64

	// sizeBytes defines the total size of the mempool (sum of all tx bytes)
	sizeBytes int64

	// cache defines a fixed-size cache of already seen transactions as this
	// reduces pressure on the proxyApp.
	cache mempool.TxCache

	// txStore defines the main storage of valid transactions. Indexes are built
	// on top of this store.
	txStore *TxStore

	// gossipIndex defines the gossiping index of valid transactions via a
	// thread-safe linked-list. We also use the gossip index as a cursor for
	// rechecking transactions already in the mempool.
	gossipIndex *clist.CList

	// recheckCursor and recheckEnd are used as cursors based on the gossip index
	// to recheck transactions that are already in the mempool. Iteration is not
	// thread-safe and transaction may be mutated in serial order.
	//
	// XXX/TODO: It might be somewhat of a codesmell to use the gossip index for
	// iterator and cursor management when rechecking transactions. If the gossip
	// index changes or is removed in a future refactor, this will have to be
	// refactored. Instead, we should consider just keeping a slice of a snapshot
	// of the mempool's current transactions during Update and an integer cursor
	// into that slice. This, however, requires additional O(n) space complexity.
	recheckCursor *clist.CElement // next expected response
	recheckEnd    *clist.CElement // re-checking stops here

	// priorityIndex defines the priority index of valid transactions via a
	// thread-safe priority queue.
	priorityIndex *TxPriorityQueue

	// heightIndex defines a height-based, in ascending order, transaction index.
	// i.e. older transactions are first.
	heightIndex *WrappedTxList

	// timestampIndex defines a timestamp-based, in ascending order, transaction
	// index. i.e. older transactions are first.
	timestampIndex *WrappedTxList

	// A read/write lock is used to safe guard updates, insertions and deletions
	// from the mempool. A read-lock is implicitly acquired when executing CheckTx,
	// however, a caller must explicitly grab a write-lock via Lock when updating
	// the mempool via Update().
	mtx       tmsync.RWMutex
	preCheck  mempool.PreCheckFunc
	postCheck mempool.PostCheckFunc
}

func NewTxMempool(
	logger log.Logger,
	cfg *config.MempoolConfig,
	proxyAppConn proxy.AppConnMempool,
	height int64,
	options ...TxMempoolOption,
) *TxMempool {

	txmp := &TxMempool{
		logger:        logger,
		config:        cfg,
		proxyAppConn:  proxyAppConn,
		height:        height,
		cache:         mempool.NopTxCache{},
		metrics:       mempool.NopMetrics(),
		txStore:       NewTxStore(),
		gossipIndex:   clist.New(),
		priorityIndex: NewTxPriorityQueue(),
		heightIndex: NewWrappedTxList(func(wtx1, wtx2 *WrappedTx) bool {
			return wtx1.height >= wtx2.height
		}),
		timestampIndex: NewWrappedTxList(func(wtx1, wtx2 *WrappedTx) bool {
			return wtx1.timestamp.After(wtx2.timestamp) || wtx1.timestamp.Equal(wtx2.timestamp)
		}),
	}

	if cfg.CacheSize > 0 {
		txmp.cache = mempool.NewLRUTxCache(cfg.CacheSize)
	}

	proxyAppConn.SetResponseCallback(txmp.defaultTxCallback)

	for _, opt := range options {
		opt(txmp)
	}

	return txmp
}

// WithPreCheck sets a filter for the mempool to reject a transaction if f(tx)
// returns an error. This is executed before CheckTx. It only applies to the
// first created block. After that, Update() overwrites the existing value.
func WithPreCheck(f mempool.PreCheckFunc) TxMempoolOption {
	return func(txmp *TxMempool) { txmp.preCheck = f }
}

// WithPostCheck sets a filter for the mempool to reject a transaction if
// f(tx, resp) returns an error. This is executed after CheckTx. It only applies
// to the first created block. After that, Update overwrites the existing value.
func WithPostCheck(f mempool.PostCheckFunc) TxMempoolOption {
	return func(txmp *TxMempool) { txmp.postCheck = f }
}

// WithMetrics sets the mempool's metrics collector.
func WithMetrics(metrics *mempool.Metrics) TxMempoolOption {
	return func(txmp *TxMempool) { txmp.metrics = metrics }
}

// Lock obtains a write-lock on the mempool. A caller must be sure to explicitly
// release the lock when finished.
func (txmp *TxMempool) Lock() {
	txmp.mtx.Lock()
}

// Unlock releases a write-lock on the mempool.
func (txmp *TxMempool) Unlock() {
	txmp.mtx.Unlock()
}

// Size returns the number of valid transactions in the mempool. It is
// thread-safe.
func (txmp *TxMempool) Size() int {
	return txmp.txStore.Size()
}

// SizeBytes return the total sum in bytes of all the valid transactions in the
// mempool. It is thread-safe.
func (txmp *TxMempool) SizeBytes() int64 {
	return atomic.LoadInt64(&txmp.sizeBytes)
}

// FlushAppConn executes FlushSync on the mempool's proxyAppConn.
//
// NOTE: The caller must obtain a write-lock via Lock() prior to execution.
func (txmp *TxMempool) FlushAppConn() error {
	return txmp.proxyAppConn.FlushSync(context.Background())
}

// WaitForNextTx returns a blocking channel that will be closed when the next
// valid transaction is available to gossip. It is thread-safe.
func (txmp *TxMempool) WaitForNextTx() <-chan struct{} {
	return txmp.gossipIndex.WaitChan()
}

// NextGossipTx returns the next valid transaction to gossip. A caller must wait
// for WaitForNextTx to signal a transaction is available to gossip first. It is
// thread-safe.
func (txmp *TxMempool) NextGossipTx() *clist.CElement {
	return txmp.gossipIndex.Front()
}

// EnableTxsAvailable enables the mempool to trigger events when transactions
// are available on a block by block basis.
func (txmp *TxMempool) EnableTxsAvailable() {
	txmp.mtx.Lock()
	defer txmp.mtx.Unlock()

	txmp.txsAvailable = make(chan struct{}, 1)
}

// TxsAvailable returns a channel which fires once for every height, and only
// when transactions are available in the mempool. It is thread-safe.
func (txmp *TxMempool) TxsAvailable() <-chan struct{} {
	return txmp.txsAvailable
}

// CheckTx executes the ABCI CheckTx method for a given transaction. It acquires
// a read-lock attempts to execute the application's CheckTx ABCI method via
// CheckTxAsync. We return an error if any of the following happen:
//
// - The CheckTxAsync execution fails.
// - The transaction already exists in the cache and we've already received the
//   transaction from the peer. Otherwise, if it solely exists in the cache, we
//   return nil.
// - The transaction size exceeds the maximum transaction size as defined by the
//   configuration provided to the mempool.
// - The transaction fails Pre-Check (if it is defined).
// - The proxyAppConn fails, e.g. the buffer is full.
//
// If the mempool is full, we still execute CheckTx and attempt to find a lower
// priority transaction to evict. If such a transaction exists, we remove the
// lower priority transaction and add the new one with higher priority.
//
// NOTE:
// - The applications' CheckTx implementation may panic.
// - The caller is not to explicitly require any locks for executing CheckTx.
func (txmp *TxMempool) CheckTx(
	ctx context.Context,
	tx types.Tx,
	cb func(*abci.Response),
	txInfo mempool.TxInfo,
) error {

	txmp.mtx.RLock()
	defer txmp.mtx.RUnlock()

	txSize := len(tx)
	if txSize > txmp.config.MaxTxBytes {
		return types.ErrTxTooLarge{
			Max:    txmp.config.MaxTxBytes,
			Actual: txSize,
		}
	}

	if txmp.preCheck != nil {
		if err := txmp.preCheck(tx); err != nil {
			return types.ErrPreCheck{
				Reason: err,
			}
		}
	}

	if err := txmp.proxyAppConn.Error(); err != nil {
		return err
	}

	txHash := tx.Key()

	// We add the transaction to the mempool's cache and if the transaction already
	// exists, i.e. false is returned, then we check if we've seen this transaction
	// from the same sender and error if we have. Otherwise, we return nil.
	if !txmp.cache.Push(tx) {
		wtx, ok := txmp.txStore.GetOrSetPeerByTxHash(txHash, txInfo.SenderID)
		if wtx != nil && ok {
			// We already have the transaction stored and the we've already seen this
			// transaction from txInfo.SenderID.
			return types.ErrTxInCache
		}

		txmp.logger.Debug("tx exists already in cache", "tx_hash", tx.Hash())
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	reqRes, err := txmp.proxyAppConn.CheckTxAsync(ctx, abci.RequestCheckTx{Tx: tx})
	if err != nil {
		txmp.cache.Remove(tx)
		return err
	}

	reqRes.SetCallback(func(res *abci.Response) {
		if txmp.recheckCursor != nil {
			panic("recheck cursor is non-nil in CheckTx callback")
		}

		wtx := &WrappedTx{
			tx:        tx,
			hash:      txHash,
			timestamp: time.Now().UTC(),
			height:    txmp.height,
		}
		txmp.initTxCallback(wtx, res, txInfo)

		if cb != nil {
			cb(res)
		}
	})

	return nil
}

func (txmp *TxMempool) RemoveTxByKey(txKey types.TxKey) error {
	txmp.Lock()
	defer txmp.Unlock()

	// remove the committed transaction from the transaction store and indexes
	if wtx := txmp.txStore.GetTxByHash(txKey); wtx != nil {
		txmp.removeTx(wtx, false)
		return nil
	}

	return errors.New("transaction not found")
}

// Flush flushes out the mempool. It acquires a read-lock, fetches all the
// transactions currently in the transaction store and removes each transaction
// from the store and all indexes and finally resets the cache.
//
// NOTE:
// - Flushing the mempool may leave the mempool in an inconsistent state.
func (txmp *TxMempool) Flush() {
	txmp.mtx.RLock()
	defer txmp.mtx.RUnlock()

	txmp.heightIndex.Reset()
	txmp.timestampIndex.Reset()

	for _, wtx := range txmp.txStore.GetAllTxs() {
		txmp.removeTx(wtx, false)
	}

	atomic.SwapInt64(&txmp.sizeBytes, 0)
	txmp.cache.Reset()
}

// ReapMaxBytesMaxGas returns a list of transactions within the provided size
// and gas constraints. Transaction are retrieved in priority order.
//
// NOTE:
// - A read-lock is acquired.
// - Transactions returned are not actually removed from the mempool transaction
//   store or indexes.
func (txmp *TxMempool) ReapMaxBytesMaxGas(maxBytes, maxGas int64) types.Txs {
	txmp.mtx.RLock()
	defer txmp.mtx.RUnlock()

	var (
		totalGas  int64
		totalSize int64
	)

	// wTxs contains a list of *WrappedTx retrieved from the priority queue that
	// need to be re-enqueued prior to returning.
	wTxs := make([]*WrappedTx, 0, txmp.priorityIndex.NumTxs())
	defer func() {
		for _, wtx := range wTxs {
			txmp.priorityIndex.PushTx(wtx)
		}
	}()

	txs := make([]types.Tx, 0, txmp.priorityIndex.NumTxs())
	for txmp.priorityIndex.NumTxs() > 0 {
		wtx := txmp.priorityIndex.PopTx()
		txs = append(txs, wtx.tx)
		wTxs = append(wTxs, wtx)
		size := types.ComputeProtoSizeForTxs([]types.Tx{wtx.tx})

		// Ensure we have capacity for the transaction with respect to the
		// transaction size.
		if maxBytes > -1 && totalSize+size > maxBytes {
			return txs[:len(txs)-1]
		}

		totalSize += size

		// ensure we have capacity for the transaction with respect to total gas
		gas := totalGas + wtx.gasWanted
		if maxGas > -1 && gas > maxGas {
			return txs[:len(txs)-1]
		}

		totalGas = gas
	}

	return txs
}

// ReapMaxTxs returns a list of transactions within the provided number of
// transactions bound. Transaction are retrieved in priority order.
//
// NOTE:
// - A read-lock is acquired.
// - Transactions returned are not actually removed from the mempool transaction
//   store or indexes.
func (txmp *TxMempool) ReapMaxTxs(max int) types.Txs {
	txmp.mtx.RLock()
	defer txmp.mtx.RUnlock()

	numTxs := txmp.priorityIndex.NumTxs()
	if max < 0 {
		max = numTxs
	}

	cap := tmmath.MinInt(numTxs, max)

	// wTxs contains a list of *WrappedTx retrieved from the priority queue that
	// need to be re-enqueued prior to returning.
	wTxs := make([]*WrappedTx, 0, cap)
	defer func() {
		for _, wtx := range wTxs {
			txmp.priorityIndex.PushTx(wtx)
		}
	}()

	txs := make([]types.Tx, 0, cap)
	for txmp.priorityIndex.NumTxs() > 0 && len(txs) < max {
		wtx := txmp.priorityIndex.PopTx()
		txs = append(txs, wtx.tx)
		wTxs = append(wTxs, wtx)
	}

	return txs
}

// Update iterates over all the transactions provided by the caller, i.e. the
// block producer, and removes them from the cache (if applicable) and removes
// the transactions from the main transaction store and associated indexes.
// Finally, if there are trainsactions remaining in the mempool, we initiate a
// re-CheckTx for them (if applicable), otherwise, we notify the caller more
// transactions are available.
//
// NOTE:
// - The caller must explicitly acquire a write-lock via Lock().
func (txmp *TxMempool) Update(
	blockHeight int64,
	blockTxs types.Txs,
	deliverTxResponses []*abci.ResponseDeliverTx,
	newPreFn mempool.PreCheckFunc,
	newPostFn mempool.PostCheckFunc,
) error {

	txmp.height = blockHeight
	txmp.notifiedTxsAvailable = false

	if newPreFn != nil {
		txmp.preCheck = newPreFn
	}
	if newPostFn != nil {
		txmp.postCheck = newPostFn
	}

	for i, tx := range blockTxs {
		if deliverTxResponses[i].Code == abci.CodeTypeOK {
			// add the valid committed transaction to the cache (if missing)
			_ = txmp.cache.Push(tx)
		} else if !txmp.config.KeepInvalidTxsInCache {
			// allow invalid transactions to be re-submitted
			txmp.cache.Remove(tx)
		}

		// remove the committed transaction from the transaction store and indexes
		if wtx := txmp.txStore.GetTxByHash(tx.Key()); wtx != nil {
			txmp.removeTx(wtx, false)
		}
	}

	txmp.purgeExpiredTxs(blockHeight)

	// If there any uncommitted transactions left in the mempool, we either
	// initiate re-CheckTx per remaining transaction or notify that remaining
	// transactions are left.
	if txmp.Size() > 0 {
		if txmp.config.Recheck {
			txmp.logger.Debug(
				"executing re-CheckTx for all remaining transactions",
				"num_txs", txmp.Size(),
				"height", blockHeight,
			)
			txmp.updateReCheckTxs()
		} else {
			txmp.notifyTxsAvailable()
		}
	}

	txmp.metrics.Size.Set(float64(txmp.Size()))
	return nil
}

// initTxCallback performs the initial, i.e. the first, callback after CheckTx
// has been executed by the ABCI application. In other words, initTxCallback is
// called after executing CheckTx when we see a unique transaction for the first
// time. CheckTx can be called again for the same transaction at a later point
// in time when re-checking, however, this callback will not be called.
//
// After the ABCI application executes CheckTx, initTxCallback is called with
// the ABCI *Response object and TxInfo. If postCheck is defined on the mempool,
// we execute that first. If there is no error from postCheck (if defined) and
// the ABCI CheckTx response code is OK, we attempt to insert the transaction.
//
// When attempting to insert the transaction, we first check if there is
// sufficient capacity. If there is sufficient capacity, the transaction is
// inserted into the txStore and indexed across all indexes. Otherwise, if the
// mempool is full, we attempt to find a lower priority transaction to evict in
// place of the new incoming transaction. If no such transaction exists, the
// new incoming transaction is rejected.
//
// If the new incoming transaction fails CheckTx or postCheck fails, we reject
// the new incoming transaction.
//
// NOTE:
// - An explicit lock is NOT required.
func (txmp *TxMempool) initTxCallback(wtx *WrappedTx, res *abci.Response, txInfo mempool.TxInfo) {
	checkTxRes, ok := res.Value.(*abci.Response_CheckTx)
	if !ok {
		return
	}

	var err error
	if txmp.postCheck != nil {
		err = txmp.postCheck(wtx.tx, checkTxRes.CheckTx)
	}

	if err != nil || checkTxRes.CheckTx.Code != abci.CodeTypeOK {
		// ignore bad transactions
		txmp.logger.Info(
			"rejected bad transaction",
			"priority", wtx.priority,
			"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
			"peer_id", txInfo.SenderNodeID,
			"code", checkTxRes.CheckTx.Code,
			"post_check_err", err,
		)

		txmp.metrics.FailedTxs.Add(1)

		if !txmp.config.KeepInvalidTxsInCache {
			txmp.cache.Remove(wtx.tx)
		}
		if err != nil {
			checkTxRes.CheckTx.MempoolError = err.Error()
		}
		return
	}

	sender := checkTxRes.CheckTx.Sender
	priority := checkTxRes.CheckTx.Priority

	if len(sender) > 0 {
		if wtx := txmp.txStore.GetTxBySender(sender); wtx != nil {
			txmp.logger.Error(
				"rejected incoming good transaction; tx already exists for sender",
				"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
				"sender", sender,
			)
			txmp.metrics.RejectedTxs.Add(1)
			return
		}
	}

	if err := txmp.canAddTx(wtx); err != nil {
		evictTxs := txmp.priorityIndex.GetEvictableTxs(
			priority,
			int64(wtx.Size()),
			txmp.SizeBytes(),
			txmp.config.MaxTxsBytes,
		)
		if len(evictTxs) == 0 {
			// No room for the new incoming transaction so we just remove it from
			// the cache.
			txmp.cache.Remove(wtx.tx)
			txmp.logger.Error(
				"rejected incoming good transaction; mempool full",
				"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
				"err", err.Error(),
			)
			txmp.metrics.RejectedTxs.Add(1)
			return
		}

		// evict an existing transaction(s)
		//
		// NOTE:
		// - The transaction, toEvict, can be removed while a concurrent
		//   reCheckTx callback is being executed for the same transaction.
		for _, toEvict := range evictTxs {
			txmp.removeTx(toEvict, true)
			txmp.logger.Debug(
				"evicted existing good transaction; mempool full",
				"old_tx", fmt.Sprintf("%X", toEvict.tx.Hash()),
				"old_priority", toEvict.priority,
				"new_tx", fmt.Sprintf("%X", wtx.tx.Hash()),
				"new_priority", wtx.priority,
			)
			txmp.metrics.EvictedTxs.Add(1)
		}
	}

	wtx.gasWanted = checkTxRes.CheckTx.GasWanted
	wtx.priority = priority
	wtx.sender = sender
	wtx.peers = map[uint16]struct{}{
		txInfo.SenderID: {},
	}

	txmp.metrics.TxSizeBytes.Observe(float64(wtx.Size()))
	txmp.metrics.Size.Set(float64(txmp.Size()))

	txmp.insertTx(wtx)
	txmp.logger.Debug(
		"inserted good transaction",
		"priority", wtx.priority,
		"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
		"height", txmp.height,
		"num_txs", txmp.Size(),
	)
	txmp.notifyTxsAvailable()

}

// defaultTxCallback performs the default CheckTx application callback. This is
// NOT executed when a transaction is first seen/received. Instead, this callback
// is executed during re-checking transactions (if enabled). A caller, i.e a
// block proposer, acquires a mempool write-lock via Lock() and when executing
// Update(), if the mempool is non-empty and Recheck is enabled, then all
// remaining transactions will be rechecked via CheckTxAsync. The order in which
// they are rechecked must be the same order in which this callback is called
// per transaction.
func (txmp *TxMempool) defaultTxCallback(req *abci.Request, res *abci.Response) {
	if txmp.recheckCursor == nil {
		return
	}

	txmp.metrics.RecheckTimes.Add(1)

	checkTxRes, ok := res.Value.(*abci.Response_CheckTx)
	if !ok {
		txmp.logger.Error("received incorrect type in mempool callback",
			"expected", reflect.TypeOf(&abci.Response_CheckTx{}).Name(),
			"got", reflect.TypeOf(res.Value).Name(),
		)
		return
	}
	tx := req.GetCheckTx().Tx
	wtx := txmp.recheckCursor.Value.(*WrappedTx)

	// Search through the remaining list of tx to recheck for a transaction that matches
	// the one we received from the ABCI application.
	for {
		if bytes.Equal(tx, wtx.tx) {
			// We've found a tx in the recheck list that matches the tx that we
			// received from the ABCI application.
			// Break, and use this transaction for further checks.
			break
		}

		txmp.logger.Error(
			"re-CheckTx transaction mismatch",
			"got", wtx.tx.Hash(),
			"expected", types.Tx(tx).Key(),
		)

		if txmp.recheckCursor == txmp.recheckEnd {
			// we reached the end of the recheckTx list without finding a tx
			// matching the one we received from the ABCI application.
			// Return without processing any tx.
			txmp.recheckCursor = nil
			return
		}

		txmp.recheckCursor = txmp.recheckCursor.Next()
		wtx = txmp.recheckCursor.Value.(*WrappedTx)
	}

	// Only evaluate transactions that have not been removed. This can happen
	// if an existing transaction is evicted during CheckTx and while this
	// callback is being executed for the same evicted transaction.
	if !txmp.txStore.IsTxRemoved(wtx.hash) {
		var err error
		if txmp.postCheck != nil {
			err = txmp.postCheck(tx, checkTxRes.CheckTx)
		}

		if checkTxRes.CheckTx.Code == abci.CodeTypeOK && err == nil {
			wtx.priority = checkTxRes.CheckTx.Priority
		} else {
			txmp.logger.Debug(
				"existing transaction no longer valid; failed re-CheckTx callback",
				"priority", wtx.priority,
				"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
				"err", err,
				"code", checkTxRes.CheckTx.Code,
			)

			if wtx.gossipEl != txmp.recheckCursor {
				panic("corrupted reCheckTx cursor")
			}

			txmp.removeTx(wtx, !txmp.config.KeepInvalidTxsInCache)
		}
	}

	// move reCheckTx cursor to next element
	if txmp.recheckCursor == txmp.recheckEnd {
		txmp.recheckCursor = nil
	} else {
		txmp.recheckCursor = txmp.recheckCursor.Next()
	}

	if txmp.recheckCursor == nil {
		txmp.logger.Debug("finished rechecking transactions")

		if txmp.Size() > 0 {
			txmp.notifyTxsAvailable()
		}
	}

	txmp.metrics.Size.Set(float64(txmp.Size()))
}

// updateReCheckTxs updates the recheck cursors by using the gossipIndex. For
// each transaction, it executes CheckTxAsync. The global callback defined on
// the proxyAppConn will be executed for each transaction after CheckTx is
// executed.
//
// NOTE:
// - The caller must have a write-lock when executing updateReCheckTxs.
func (txmp *TxMempool) updateReCheckTxs() {
	if txmp.Size() == 0 {
		panic("attempted to update re-CheckTx txs when mempool is empty")
	}

	txmp.recheckCursor = txmp.gossipIndex.Front()
	txmp.recheckEnd = txmp.gossipIndex.Back()
	ctx := context.Background()

	for e := txmp.gossipIndex.Front(); e != nil; e = e.Next() {
		wtx := e.Value.(*WrappedTx)

		// Only execute CheckTx if the transaction is not marked as removed which
		// could happen if the transaction was evicted.
		if !txmp.txStore.IsTxRemoved(wtx.hash) {
			_, err := txmp.proxyAppConn.CheckTxAsync(ctx, abci.RequestCheckTx{
				Tx:   wtx.tx,
				Type: abci.CheckTxType_Recheck,
			})
			if err != nil {
				// no need in retrying since the tx will be rechecked after the next block
				txmp.logger.Error("failed to execute CheckTx during rechecking", "err", err)
			}
		}
	}

	if _, err := txmp.proxyAppConn.FlushAsync(ctx); err != nil {
		txmp.logger.Error("failed to flush transactions during rechecking", "err", err)
	}
}

// canAddTx returns an error if we cannot insert the provided *WrappedTx into
// the mempool due to mempool configured constraints. Otherwise, nil is returned
// and the transaction can be inserted into the mempool.
func (txmp *TxMempool) canAddTx(wtx *WrappedTx) error {
	var (
		numTxs    = txmp.Size()
		sizeBytes = txmp.SizeBytes()
	)

	if numTxs >= txmp.config.Size || int64(wtx.Size())+sizeBytes > txmp.config.MaxTxsBytes {
		return types.ErrMempoolIsFull{
			NumTxs:      numTxs,
			MaxTxs:      txmp.config.Size,
			TxsBytes:    sizeBytes,
			MaxTxsBytes: txmp.config.MaxTxsBytes,
		}
	}

	return nil
}

func (txmp *TxMempool) insertTx(wtx *WrappedTx) {
	txmp.txStore.SetTx(wtx)
	txmp.priorityIndex.PushTx(wtx)
	txmp.heightIndex.Insert(wtx)
	txmp.timestampIndex.Insert(wtx)

	// Insert the transaction into the gossip index and mark the reference to the
	// linked-list element, which will be needed at a later point when the
	// transaction is removed.
	gossipEl := txmp.gossipIndex.PushBack(wtx)
	wtx.gossipEl = gossipEl

	atomic.AddInt64(&txmp.sizeBytes, int64(wtx.Size()))
}

func (txmp *TxMempool) removeTx(wtx *WrappedTx, removeFromCache bool) {
	if txmp.txStore.IsTxRemoved(wtx.hash) {
		return
	}

	txmp.txStore.RemoveTx(wtx)
	txmp.priorityIndex.RemoveTx(wtx)
	txmp.heightIndex.Remove(wtx)
	txmp.timestampIndex.Remove(wtx)

	// Remove the transaction from the gossip index and cleanup the linked-list
	// element so it can be garbage collected.
	txmp.gossipIndex.Remove(wtx.gossipEl)
	wtx.gossipEl.DetachPrev()

	atomic.AddInt64(&txmp.sizeBytes, int64(-wtx.Size()))

	if removeFromCache {
		txmp.cache.Remove(wtx.tx)
	}
}

// purgeExpiredTxs removes all transactions that have exceeded their respective
// height and/or time based TTLs from their respective indexes. Every expired
// transaction will be removed from the mempool entirely, except for the cache.
//
// NOTE: purgeExpiredTxs must only be called during TxMempool#Update in which
// the caller has a write-lock on the mempool and so we can safely iterate over
// the height and time based indexes.
func (txmp *TxMempool) purgeExpiredTxs(blockHeight int64) {
	now := time.Now()
	expiredTxs := make(map[types.TxKey]*WrappedTx)

	if txmp.config.TTLNumBlocks > 0 {
		purgeIdx := -1
		for i, wtx := range txmp.heightIndex.txs {
			if (blockHeight - wtx.height) > txmp.config.TTLNumBlocks {
				expiredTxs[wtx.tx.Key()] = wtx
				purgeIdx = i
			} else {
				// since the index is sorted, we know no other txs can be be purged
				break
			}
		}

		if purgeIdx >= 0 {
			txmp.heightIndex.txs = txmp.heightIndex.txs[purgeIdx+1:]
		}
	}

	if txmp.config.TTLDuration > 0 {
		purgeIdx := -1
		for i, wtx := range txmp.timestampIndex.txs {
			if now.Sub(wtx.timestamp) > txmp.config.TTLDuration {
				expiredTxs[wtx.tx.Key()] = wtx
				purgeIdx = i
			} else {
				// since the index is sorted, we know no other txs can be be purged
				break
			}
		}

		if purgeIdx >= 0 {
			txmp.timestampIndex.txs = txmp.timestampIndex.txs[purgeIdx+1:]
		}
	}

	for _, wtx := range expiredTxs {
		txmp.removeTx(wtx, false)
	}
}

func (txmp *TxMempool) notifyTxsAvailable() {
	if txmp.Size() == 0 {
		panic("attempt to notify txs available but mempool is empty!")
	}

	if txmp.txsAvailable != nil && !txmp.notifiedTxsAvailable {
		// channel cap is 1, so this will send once
		txmp.notifiedTxsAvailable = true

		select {
		case txmp.txsAvailable <- struct{}{}:
		default:
		}
	}
}
