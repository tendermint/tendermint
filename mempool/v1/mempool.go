package v1

import (
	"context"
	"sync/atomic"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
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
	// thread-safe and transaction may be mutated in serial.
	recheckCursor *clist.CElement // next expected response
	recheckEnd    *clist.CElement // re-checking stops here

	// priorityIndex defines the priority index of valid transactions via a
	// thread-safe priority queue.
	priorityIndex *TxPriorityQueue

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

	mp := &TxMempool{
		logger:        logger,
		config:        cfg,
		proxyAppConn:  proxyAppConn,
		height:        height,
		cache:         mempool.NopTxCache{},
		metrics:       mempool.NopMetrics(),
		txStore:       NewTxStore(),
		gossipIndex:   clist.New(),
		priorityIndex: NewTxPriorityQueue(),
	}

	if cfg.CacheSize > 0 {
		mp.cache = mempool.NewLRUTxCache(cfg.CacheSize)
	}

	// TODO:
	// proxyAppConn.SetResponseCallback(mp.globalCb)

	for _, opt := range options {
		opt(mp)
	}

	return mp
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
// thread-safe and uses the underlying gossip index to infer the total number of
// transactions.
func (txmp *TxMempool) Size() int {
	return txmp.gossipIndex.Len()
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
func (txmp *TxMempool) NextGossipTx() *WrappedTx {
	return txmp.gossipIndex.Front().Value.(*WrappedTx)
}

// EnableTxsAvailable enables the mempool to trigger events when transactions
// are available on a block by block basis.
//
// NOTE: It is NOT thread-safe and should only be called once on startup.
func (txmp *TxMempool) EnableTxsAvailable() {
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
func (txmp *TxMempool) CheckTx(tx types.Tx, cb func(*abci.Response), txInfo mempool.TxInfo) error {
	txmp.mtx.RLock()
	defer txmp.mtx.RUnlock()

	txSize := len(tx)
	if txSize > txmp.config.MaxTxBytes {
		return mempool.ErrTxTooLarge{
			Max:    txmp.config.MaxTxBytes,
			Actual: txSize,
		}
	}

	if txmp.preCheck != nil {
		if err := txmp.preCheck(tx); err != nil {
			return mempool.ErrPreCheck{
				Reason: err,
			}
		}
	}

	if err := txmp.proxyAppConn.Error(); err != nil {
		return err
	}

	// We add the transaction to the mempool's cache and if the transaction already
	// exists, i.e. false is returned, then we check if we've seen this transaction
	// from the same sender and error if we have. Otherwise, we return nil.
	if !txmp.cache.Push(tx) {
		wtx, ok := txmp.txStore.GetOrSetPeerByTxHash(mempool.TxKey(tx), txInfo.SenderID)
		if wtx != nil && ok {
			// We already have the transaction stored and the we've already seen this
			// transaction from txInfo.SenderID.
			return mempool.ErrTxInCache
		}

		txmp.logger.Debug("tx exists already in cache", "tx_hash", tx.Hash())
		return nil
	}

	ctx := txInfo.Context
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
			Tx:        tx,
			Timestamp: time.Now(),
		}
		txmp.initTxCallback(wtx, res, txInfo)

		if cb != nil {
			cb(res)
		}
	})

	return nil
}

func (txmp *TxMempool) Flush() {
	panic("not implemented")
}

func (txmp *TxMempool) ReapMaxBytesMaxGas(maxBytes, maxGas int64) types.Txs {
	panic("not implemented")
}

func (txmp *TxMempool) ReapMaxTxs(max int) types.Txs {
	panic("not implemented")
}

func (txmp *TxMempool) Update(
	blockHeight int64,
	blockTxs types.Txs,
	deliverTxResponses []*abci.ResponseDeliverTx,
	newPreFn mempool.PreCheckFunc,
	newPostFn mempool.PostCheckFunc,
) error {
	panic("not implemented")
}

func (txmp *TxMempool) initTxCallback(wtx *WrappedTx, res *abci.Response, txInfo mempool.TxInfo) {
	checkTxRes, ok := res.Value.(*abci.Response_CheckTx)
	if ok {
		var err error
		if txmp.postCheck != nil {
			err = txmp.postCheck(wtx.Tx, checkTxRes.CheckTx)
		}

		if checkTxRes.CheckTx.Code == abci.CodeTypeOK && err == nil {
			if err := txmp.canAddTx(wtx); err != nil {
				toEvict := txmp.priorityIndex.GetEvictableTx(checkTxRes.CheckTx.Priority)
				if toEvict == nil {
					// no remove for the incoming transaction
					txmp.cache.Remove(wtx.Tx)
					txmp.logger.Error("rejected good transaction; mempool full", "err", err.Error())
					return
				} else {
					// we have the ability to evict an existing transaction
					// TODO: ...
				}
			}

			wtx.Priority = checkTxRes.CheckTx.Priority
			wtx.Sender = checkTxRes.CheckTx.Sender

			txmp.insertTx(wtx)
			txmp.notifyTxsAvailable()

		} else {
			// ignore bad transactions
			txmp.logger.Debug(
				"rejected bad transaction",
				"tx", mempool.TxHashFromBytes(wtx.Tx),
				"peer_id", txInfo.SenderNodeID,
				"res", checkTxRes,
				"post_check_err", err,
			)

			txmp.metrics.FailedTxs.Add(1)

			if !txmp.config.KeepInvalidTxsInCache {
				txmp.cache.Remove(wtx.Tx)
			}
		}
	}

	txmp.metrics.Size.Set(float64(txmp.Size()))
}

func (txmp *TxMempool) canAddTx(wtx *WrappedTx) error {
	var (
		numTxs    = txmp.Size()
		sizeBytes = txmp.SizeBytes()
	)

	if numTxs >= txmp.config.Size || int64(wtx.Size())+sizeBytes > txmp.config.MaxTxsBytes {
		return mempool.ErrMempoolIsFull{
			NumTxs:      numTxs,
			MaxTxs:      txmp.config.Size,
			TxsBytes:    sizeBytes,
			MaxTxsBytes: txmp.config.MaxTxsBytes,
		}
	}

	return nil
}

func (txmp *TxMempool) insertTx(wtx *WrappedTx) {
	// insert the transaction into the main store and set indices
	txmp.txStore.SetTx(wtx)
	txmp.gossipIndex.PushBack(wtx)
	txmp.priorityIndex.PushTx(wtx)

	atomic.AddInt64(&txmp.sizeBytes, int64(wtx.Size()))
	txmp.metrics.TxSizeBytes.Observe(float64(wtx.Size()))

	txmp.logger.Debug(
		"added good transaction",
		"tx", mempool.TxHashFromBytes(wtx.Tx),
		"height", txmp.height,
		"num_txs", txmp.Size(),
	)
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
