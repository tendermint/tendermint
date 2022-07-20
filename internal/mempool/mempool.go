package mempool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/types"
)

var _ Pool = (*TxMempool)(nil)

// TxMempoolOption sets an optional parameter on the TxMempool.
type TxMempoolOption func(*TxMempool)

// TxMempool defines a prioritized mempool data structure used by the v1 mempool
// reactor. It keeps a thread-safe priority queue of transactions that is used
// when a block proposer constructs a block and a thread-safe linked-list that
// is used to gossip transactions to peers in a FIFO manner.
type TxMempool struct {
	logger       log.Logger
	metrics      *Metrics
	config       *config.MempoolConfig
	proxyAppConn abciclient.Client

	// height defines the last block height process during Update()
	height int64

	// szBytes defines the total size of the mempool (sum of all tx bytes)
	szBytes int64

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
	mtx       sync.RWMutex
	postCheck PostCheckFunc
}

func NewTxMempool(
	logger log.Logger,
	cfg *config.MempoolConfig,
	proxyAppConn abciclient.Client,
	options ...TxMempoolOption,
) *TxMempool {

	txmp := &TxMempool{
		logger:        logger,
		config:        cfg,
		proxyAppConn:  proxyAppConn,
		height:        -1,
		metrics:       NopMetrics(),
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

	for _, opt := range options {
		opt(txmp)
	}

	return txmp
}

// WithPostCheck sets a filter for the mempool to reject a transaction if
// f(tx, resp) returns an error. This is executed after CheckTx. It only applies
// to the first created block. After that, Update overwrites the existing value.
func WithPostCheck(f PostCheckFunc) TxMempoolOption {
	return func(txmp *TxMempool) { txmp.postCheck = f }
}

// WithMetrics sets the mempool's metrics collector.
func WithMetrics(metrics *Metrics) TxMempoolOption {
	return func(txmp *TxMempool) { txmp.metrics = metrics }
}

// Meta returns the tx mempool's metatdata.
func (txmp *TxMempool) Meta() PoolMeta {
	return PoolMeta{
		Type:       "priority",
		Size:       txmp.txStore.Size(),
		TotalBytes: atomic.LoadInt64(&txmp.szBytes),
	}
}

// Size returns the number of valid transactions in the mempool. It is
// thread-safe.
func (txmp *TxMempool) size() int {
	return txmp.txStore.Size()
}

// SizeBytes return the total sum in bytes of all the valid transactions in the
// mempool. It is thread-safe.
func (txmp *TxMempool) sizeBytes() int64 {
	return atomic.LoadInt64(&txmp.szBytes)
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

// CheckTXCallback satisfies the behavior the MempoolABCI client depends on from the
// Pool type. In this, our MempoolABCI client is managing the Locking and app conn.
// The CheckTXCallback only happens after the necessary abci preparations have been
// made. Removing those concerns from our store implementation.
func (txmp *TxMempool) CheckTXCallback(ctx context.Context, tx types.Tx, res *abci.ResponseCheckTx, txInfo TxInfo) (OpResult, error) {
	if txmp.recheckCursor != nil {
		return OpResult{}, errors.New("recheck cursor is non-nil")
	}

	wtx := &WrappedTx{
		tx:        tx,
		hash:      tx.Key(),
		timestamp: time.Now().UTC(),
		height:    txmp.height,
	}

	var opRes OpResult
	defCBOpRes := txmp.defaultTxCallback(tx, res)
	if defCBOpRes.Status != "" {
		opRes.Status = defCBOpRes.Status
	}

	initCBOpRes := txmp.initTxCallback(wtx, res, txInfo)
	if opRes.Status == "" && initCBOpRes.Status != "" {
		opRes.Status = initCBOpRes.Status
	}
	opRes.RemovedTXs = make(types.Txs, 0, len(defCBOpRes.RemovedTXs)+len(initCBOpRes.RemovedTXs))
	opRes.RemovedTXs = append(opRes.RemovedTXs, defCBOpRes.RemovedTXs...)
	opRes.RemovedTXs = append(opRes.RemovedTXs, initCBOpRes.RemovedTXs...)

	return opRes, nil
}

// Remove removes TXs from the underlying store.
func (txmp *TxMempool) Remove(ctx context.Context, opts ...RemOptFn) (OpResult, error) {
	opt := CoalesceRemOptFns(opts...)

	var (
		err   *multierror.Error
		opRes OpResult
	)

	for _, txKey := range opt.TXKeys {
		tx, remErr := txmp.removeTxByKey(txKey)
		if remErr != nil {
			err = multierror.Append(err, remErr)
			continue
		}
		opRes.RemovedTXs = append(opRes.RemovedTXs, tx)
	}

	return opRes, err.ErrorOrNil()
}

func (txmp *TxMempool) removeTxByKey(txKey types.TxKey) (types.Tx, error) {
	txmp.mtx.Lock()
	defer txmp.mtx.Unlock()

	// remove the committed transaction from the transaction store and indexes
	wtx := txmp.txStore.GetTxByHash(txKey)
	if wtx == nil {
		return nil, errors.New("transaction not found")
	}
	txmp.removeTx(wtx)

	return wtx.tx, nil
}

// Flush empties the mempool. It acquires a read-lock, fetches all the
// transactions currently in the transaction store and removes each transaction
// from the store and all indexes and finally resets the cache.
//
// NOTE:
// - Flushing the mempool may leave the mempool in an inconsistent state.
func (txmp *TxMempool) Flush(ctx context.Context) error {
	txmp.mtx.RLock()
	defer txmp.mtx.RUnlock()

	txmp.heightIndex.Reset()
	txmp.timestampIndex.Reset()

	for _, wtx := range txmp.txStore.GetAllTxs() {
		txmp.removeTx(wtx)
	}

	atomic.SwapInt64(&txmp.szBytes, 0)

	return nil
}

// Reap will return a list of TXs by the chose input. As of writing this, this implementation
// only supports providing either NumTXs or one of GasLimit BlockSizeLimit. Providing non
// default values for all values, will result in an error.
func (txmp *TxMempool) Reap(ctx context.Context, opts ...ReapOptFn) (types.Txs, error) {
	opt := CoalesceReapOpts(opts...)

	if opt.NumTXs > -1 && (opt.GasLimit > -1 || opt.BlockSizeLimit > -1) {
		return nil, errors.New("reaping by num txs and one of gas limit or block size limit is not supported")
	}

	if opt.NumTXs == -1 && opt.GasLimit == -1 && opt.BlockSizeLimit == -1 {
		return txmp.reapMaxTxs(-1), nil
	}

	if opt.NumTXs > -1 {
		return txmp.reapMaxTxs(opt.NumTXs), nil
	}

	return txmp.reapMaxBytesMaxGas(opt.BlockSizeLimit, opt.GasLimit), nil
}

// reapMaxBytesMaxGas returns a list of transactions within the provided size
// and gas constraints. Transaction are retrieved in priority order.
//
// NOTE:
// - Transactions returned are not removed from the mempool transaction
//   store or indexes.
func (txmp *TxMempool) reapMaxBytesMaxGas(maxBytes, maxGas int64) types.Txs {
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

// reapMaxTxs returns a list of transactions within the provided number of
// transactions bound. Transaction are retrieved in priority order.
//
// NOTE:
// - Transactions returned are not removed from the mempool transaction
//   store or indexes.
func (txmp *TxMempool) reapMaxTxs(max int) types.Txs {
	txmp.mtx.RLock()
	defer txmp.mtx.RUnlock()

	numTxs := txmp.priorityIndex.NumTxs()
	if max < 0 {
		max = numTxs
	}

	capacity := tmmath.MinInt(numTxs, max)

	// wTxs contains a list of *WrappedTx retrieved from the priority queue that
	// need to be re-enqueued prior to returning.
	wTxs := make([]*WrappedTx, 0, capacity)
	txs := make([]types.Tx, 0, capacity)
	for txmp.priorityIndex.NumTxs() > 0 && len(txs) < max {
		wtx := txmp.priorityIndex.PopTx()
		txs = append(txs, wtx.tx)
		wTxs = append(wTxs, wtx)
	}
	for _, wtx := range wTxs {
		txmp.priorityIndex.PushTx(wtx)
	}
	return txs
}

// OnUpdate fulfills the behavior the MempoolABCI client needs. The contract with the MempoolABCI
// client is that the MempoolABCI client manages locking/unlocking and all preparation for the
// update. Additionally, caching to reduce app pressure happens there as well.
func (txmp *TxMempool) OnUpdate(ctx context.Context, blockHeight int64, blockTxs types.Txs, txResults []*abci.ExecTxResult, newPostFn PostCheckFunc) (OpResult, error) {
	txmp.height = blockHeight
	if newPostFn != nil {
		txmp.postCheck = newPostFn
	}

	var opRes OpResult
	for i, tx := range blockTxs {
		if txResults[i].Code == abci.CodeTypeOK {
			// add the valid committed transaction to the cache (if missing)
			opRes.AddedTXs = append(opRes.AddedTXs, tx)
		} else if !txmp.config.KeepInvalidTxsInCache {
			// allow invalid transactions to be re-submitted
			opRes.RemovedTXs = append(opRes.RemovedTXs, tx)
		}

		// remove the committed transaction from the transaction store and indexes
		if wtx := txmp.txStore.GetTxByHash(tx.Key()); wtx != nil {
			txmp.removeTx(wtx)
		}
	}

	txmp.purgeExpiredTxs(blockHeight)

	// If there any uncommitted transactions left in the mempool, we either
	// initiate re-CheckTx per remaining transaction or notify that remaining
	// transactions are left.
	if txmp.size() > 0 {
		if txmp.config.Recheck {
			txmp.logger.Debug(
				"executing re-CheckTx for all remaining transactions",
				"num_txs", txmp.size(),
				"height", blockHeight,
			)
			updateOpRes := txmp.updateReCheckTxs(ctx)
			opRes.Status = updateOpRes.Status
			opRes.AddedTXs = append(opRes.AddedTXs, updateOpRes.AddedTXs...)
			opRes.RemovedTXs = append(opRes.RemovedTXs, updateOpRes.RemovedTXs...)
		} else {
			opRes.Status = StatusTXsAvailable
		}
	}

	txmp.metrics.Size.Set(float64(txmp.size()))

	return opRes, nil
}

// initTxCallback is the callback invoked for a new unique transaction after CheckTx
// has been executed by the ABCI application for the first time on that transaction.
// CheckTx can be called again for the same transaction later when re-checking;
// however, this callback will not be called.
//
// initTxCallback runs after the ABCI application executes CheckTx.
// It runs the postCheck hook if one is defined on the mempool.
// If the CheckTx response response code is not OK, or if the postCheck hook
// reports an error, the transaction is rejected. Otherwise, we attempt to insert
// the transaction into the mempool.
//
// When inserting a transaction, we first check if there is sufficient capacity.
// If there is, the transaction is added to the txStore and all indexes.
// Otherwise, if the mempool is full, we attempt to find a lower priority transaction
// to evict in place of the new incoming transaction. If no such transaction exists,
// the new incoming transaction is rejected.
//
// NOTE:
// - An explicit lock is NOT required.
// TODO(berg): move this into ABCI, this is likely universal, also we get to consolidate the
// 			   postCheckFn in ABCI as well as the cache
func (txmp *TxMempool) initTxCallback(wtx *WrappedTx, res *abci.ResponseCheckTx, txInfo TxInfo) OpResult {
	var err error
	if txmp.postCheck != nil {
		err = txmp.postCheck(wtx.tx, res)
	}

	if err != nil || res.Code != abci.CodeTypeOK {
		// ignore bad transactions
		txmp.logger.Info(
			"rejected bad transaction",
			"priority", wtx.priority,
			"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
			"peer_id", txInfo.SenderNodeID,
			"code", res.Code,
			"post_check_err", err,
		)

		txmp.metrics.FailedTxs.Add(1)

		if err != nil {
			res.MempoolError = err.Error()
		}
		return OpResult{RemovedTXs: types.Txs{wtx.tx}}
	}

	sender := res.Sender
	priority := res.Priority

	if len(sender) > 0 {
		if wtx := txmp.txStore.GetTxBySender(sender); wtx != nil {
			txmp.logger.Error(
				"rejected incoming good transaction; tx already exists for sender",
				"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
				"sender", sender,
			)
			txmp.metrics.RejectedTxs.Add(1)
			return OpResult{}
		}
	}

	var evictedTXs types.Txs
	if err := txmp.canAddTx(wtx); err != nil {
		evictTxs := txmp.priorityIndex.GetEvictableTxs(
			priority,
			int64(wtx.Size()),
			txmp.sizeBytes(),
			txmp.config.MaxTxsBytes,
		)
		if len(evictTxs) == 0 {
			// No room for the new incoming transaction so we just remove it from
			// the cache.
			txmp.logger.Error(
				"rejected incoming good transaction; mempool full",
				"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
				"err", err.Error(),
			)
			txmp.metrics.RejectedTxs.Add(1)
			return OpResult{RemovedTXs: types.Txs{wtx.tx}}
		}

		evictedTXs = make(types.Txs, 0, len(evictTxs))
		// evict an existing transaction(s)
		//
		// NOTE:
		// - The transaction, toEvict, can be removed while a concurrent
		//   reCheckTx callback is being executed for the same transaction.
		for _, toEvict := range evictTxs {
			evictedTXs = append(evictedTXs, toEvict.tx)
			txmp.removeTx(toEvict)
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

	wtx.gasWanted = res.GasWanted
	wtx.priority = priority
	wtx.sender = sender
	wtx.peers = map[uint16]struct{}{
		txInfo.SenderID: {},
	}

	txmp.metrics.TxSizeBytes.Observe(float64(wtx.Size()))
	txmp.metrics.Size.Set(float64(txmp.size()))

	txmp.insertTx(wtx)
	txmp.logger.Debug(
		"inserted good transaction",
		"priority", wtx.priority,
		"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
		"height", txmp.height,
		"num_txs", txmp.size(),
	)

	return OpResult{
		RemovedTXs: evictedTXs,
		Status:     StatusTXsAvailable,
	}
}

// defaultTxCallback is the CheckTx application callback used when a
// transaction is being re-checked (if re-checking is enabled). The
// caller must hold a mempool write-lock (via Lock()) and when
// executing Update(), if the mempool is non-empty and Recheck is
// enabled, then all remaining transactions will be rechecked via
// CheckTx. The order transactions are rechecked must be the same as
// the order in which this callback is called.
func (txmp *TxMempool) defaultTxCallback(tx types.Tx, res *abci.ResponseCheckTx) OpResult {
	if txmp.recheckCursor == nil {
		return OpResult{}
	}

	txmp.metrics.RecheckTimes.Add(1)

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
			"expected", tx.Key(),
		)

		if txmp.recheckCursor == txmp.recheckEnd {
			// we reached the end of the recheckTx list without finding a tx
			// matching the one we received from the ABCI application.
			// Return without processing any tx.
			txmp.recheckCursor = nil
			return OpResult{}
		}

		txmp.recheckCursor = txmp.recheckCursor.Next()
		wtx = txmp.recheckCursor.Value.(*WrappedTx)
	}

	var opRes OpResult
	// Only evaluate transactions that have not been removed. This can happen
	// if an existing transaction is evicted during CheckTx and while this
	// callback is being executed for the same evicted transaction.
	if !txmp.txStore.IsTxRemoved(wtx.hash) {
		var err error
		if txmp.postCheck != nil {
			err = txmp.postCheck(tx, res)
		}

		if res.Code == abci.CodeTypeOK && err == nil {
			wtx.priority = res.Priority
		} else {
			txmp.logger.Debug(
				"existing transaction no longer valid; failed re-CheckTx callback",
				"priority", wtx.priority,
				"tx", fmt.Sprintf("%X", wtx.tx.Hash()),
				"err", err,
				"code", res.Code,
			)

			if wtx.gossipEl != txmp.recheckCursor {
				panic("corrupted reCheckTx cursor")
			}
			opRes.RemovedTXs = append(opRes.RemovedTXs, wtx.tx)
			txmp.removeTx(wtx)
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

		if txmp.size() > 0 {
			opRes.Status = StatusTXsAvailable
		}
	}

	txmp.metrics.Size.Set(float64(txmp.size()))

	return opRes
}

// updateReCheckTxs updates the recheck cursors using the gossipIndex. For
// each transaction, it executes CheckTx. The global callback defined on
// the proxyAppConn will be executed for each transaction after CheckTx is
// executed.
//
// NOTE:
// - The caller must have a write-lock when executing updateReCheckTxs.
func (txmp *TxMempool) updateReCheckTxs(ctx context.Context) OpResult {
	if txmp.size() == 0 {
		panic("attempted to update re-CheckTx txs when mempool is empty")
	}

	txmp.recheckCursor = txmp.gossipIndex.Front()
	txmp.recheckEnd = txmp.gossipIndex.Back()

	var opRes OpResult
	for e := txmp.gossipIndex.Front(); e != nil; e = e.Next() {
		wtx := e.Value.(*WrappedTx)

		// Only execute CheckTx if the transaction is not marked as removed which
		// could happen if the transaction was evicted.
		if !txmp.txStore.IsTxRemoved(wtx.hash) {
			res, err := txmp.proxyAppConn.CheckTx(ctx, abci.RequestCheckTx{
				Tx:   wtx.tx,
				Type: abci.CheckTxType_Recheck,
			})
			if err != nil {
				// no need in retrying since the tx will be rechecked after the next block
				txmp.logger.Error("failed to execute CheckTx during rechecking", "err", err)
				continue
			}
			defCBOpRes := txmp.defaultTxCallback(wtx.tx, res)
			if opRes.Status == "" && defCBOpRes.Status != "" {
				opRes.Status = defCBOpRes.Status
			}
			opRes.RemovedTXs = append(opRes.RemovedTXs, defCBOpRes.RemovedTXs...)
		}
	}

	if err := txmp.proxyAppConn.Flush(ctx); err != nil {
		txmp.logger.Error("failed to flush transactions during rechecking", "err", err)
	}

	return opRes
}

// canAddTx returns an error if we cannot insert the provided *WrappedTx into
// the mempool due to mempool configured constraints. If it returns nil,
// the transaction can be inserted into the mempool.
func (txmp *TxMempool) canAddTx(wtx *WrappedTx) error {
	var (
		numTxs    = txmp.size()
		sizeBytes = txmp.sizeBytes()
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

	atomic.AddInt64(&txmp.szBytes, int64(wtx.Size()))
}

func (txmp *TxMempool) removeTx(wtx *WrappedTx, ) {
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

	atomic.AddInt64(&txmp.szBytes, int64(-wtx.Size()))
}

// purgeExpiredTxs removes all transactions that have exceeded their respective
// height- and/or time-based TTLs from their respective indexes. Every expired
// transaction will be removed from the mempool, but preserved in the cache.
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
		txmp.removeTx(wtx)
	}
}
