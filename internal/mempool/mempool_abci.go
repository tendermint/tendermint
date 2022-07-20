package mempool

import (
	"context"
	"fmt"
	"sync"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

// ABCI provides an ABCI integration with a mempool store. The ABCI
// client enforces the tendermint guarantees.
type ABCI struct {
	cfg                  *config.MempoolConfig
	txsAvailable         chan struct{}
	notifiedTxsAvailable bool

	// cache defines a fixed-size cache of already seen transactions as this
	// reduces pressure on the proxyApp.
	cache TxCache

	mtx     sync.RWMutex
	pool    Pool
	appConn abciclient.Client

	preCheck PreCheckFunc
}

var _ MempoolABCI = (*ABCI)(nil)

// NewABCI constructs a new ABCI type.
func NewABCI(cfg *config.MempoolConfig, appClient abciclient.Client, pool Pool, preCheck PreCheckFunc) *ABCI {
	out := ABCI{
		cfg:      cfg,
		cache:    NopTxCache{},
		pool:     pool,
		appConn:  appClient,
		preCheck: preCheck,
	}

	if size := cfg.CacheSize; size > 0 {
		out.cache = NewLRUTxCache(size)
	}

	return &out
}

// CheckTx executes the ABCI CheckTx method for a given transaction.
// It acquires a read-lock and attempts to execute the application's
// CheckTx ABCI method synchronously. We return an error if any of
// the following happen:
//
// - The CheckTx execution fails.
// - The transaction already exists in the cache and we've already received the
//   transaction from the peer. Otherwise, if it solely exists in the cache, we
//   return nil.
//   TODO(berg): the above is not accurate, we do not error on that case. It makes
//				 makes sense to include the peer state here in ABCI type and not the
//				 mempool store itself.
// - The transaction size exceeds the maximum transaction size as defined by the
//   configuration provided to the mempool.
// - The transaction fails Pre-Check (if it is defined).
// - The proxyAppConn fails, e.g. the buffer is full.
// NOTE:
// - The applications' CheckTx implementation may panic.
func (a *ABCI) CheckTx(ctx context.Context, tx types.Tx, callback func(*abci.ResponseCheckTx), txInfo TxInfo) error {
	a.mtx.RLock()
	defer a.mtx.RUnlock()

	if txSize, maxTXBytes := len(tx), a.cfg.MaxTxBytes; txSize > maxTXBytes {
		return types.ErrTxTooLarge{
			Max:    maxTXBytes,
			Actual: txSize,
		}
	}

	if err := a.runPrecheck(tx); err != nil {
		return types.ErrPreCheck{Reason: err}
	}

	if err := a.appConn.Error(); err != nil {
		return err
	}

	// We add the transaction to the mempool's cache and if the
	// transaction is already present in the cache, i.e. false is returned, then we
	// check if we've seen this transaction and error if we have.
	if !a.cache.Push(tx) {
		// TODO(berg): the below is pulled directly from TxMempool. I don't understand what
		// 			   it is doing? None of the returned types are being used? The comment
		//			   feels before the if block feels misleading. We don't check if we've
		//			   seen the tx before and error, we just call it and if it gets added
		//			   to the txstore, so be it :shruggo:
		//txmp.txStore.GetOrSetPeerByTxHash(txHash, info.SenderID)
		return types.ErrTxInCache
	}

	res, err := a.appConn.CheckTx(ctx, abci.RequestCheckTx{Tx: tx})
	if err != nil {
		// TODO(berg): need something here for failed on CheckTx
		return err
	}

	opRes, err := a.pool.CheckTXCallback(ctx, tx, res, txInfo)
	if err != nil {
		return err
	}
	a.applyOpResult(opRes)

	if callback != nil {
		callback(res)
	}

	return nil
}

// EnableTxsAvailable enables the mempool to trigger events when transactions
// are available on a block by block basis.
func (a *ABCI) EnableTxsAvailable() {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	a.txsAvailable = make(chan struct{}, 1)
}

// Flush clears the tx cache and the calls Flush on the underlying pool type.
func (a *ABCI) Flush(ctx context.Context) error {
	a.cache.Reset()
	return a.pool.Flush(ctx)
}

// PoolMeta returns the metadata for the underlying pool implementation.
func (a *ABCI) PoolMeta() PoolMeta {
	return a.pool.Meta()
}

// PrepBlockFinality prepares the mempool for an upcoming block to be finalized. During
// the execution of the new block, we want to make sure we are not running any additional
// CheckTX calls to the application as well as flush any active connections underway.
func (a *ABCI) PrepBlockFinality(ctx context.Context) (func(), error) {
	a.mtx.Lock()
	finishFn := func() { a.mtx.Unlock() }

	err := a.appConn.Flush(ctx)
	if err != nil {
		finishFn() // make sure we unlock before returning
		return func() {}, err
	}

	return finishFn, nil
}

// Reap is the sole means to reap TXs atm. Options are provided and when none are provide
// the mempool store will typically return all TXs. It is up to the store implementation
// to limit or allow that usecase.
func (a *ABCI) Reap(ctx context.Context, opts ...ReapOptFn) (types.Txs, error) {
	return a.pool.Reap(ctx, opts...)
}

func (a *ABCI) Remove(ctx context.Context, opts ...RemOptFn) error {
	opRes, err := a.pool.Remove(ctx, opts...)
	if err != nil {
		return err
	}
	a.applyOpResult(opRes)

	return nil
}

// Update iterates over all the transactions provided by the block producer,
// removes them from the cache (if applicable), and removes
// the transactions from the main transaction store and associated indexes.
// If there are transactions remaining in the mempool, we initiate a
// re-CheckTx for them (if applicable), otherwise, we notify the caller more
// transactions are available.
//
// NOTE:
// - The caller must explicitly call PrepBlockFinality, which locks the client
//	 and flushes the appconn to enforce we sync state correctly with the app
//	 on Update. Failure to do so will result in an error.
func (a *ABCI) Update(
	ctx context.Context,
	blockHeight int64,
	blockTxs types.Txs,
	txResults []*abci.ExecTxResult,
	newPreFn PreCheckFunc,
	newPostFn PostCheckFunc,
) error {
	if a.mtx.TryLock() {
		a.mtx.Unlock()
		// this TryLock call enforces the caller having to Lock the ABCI client
		// before executing the Update
		return fmt.Errorf("caller failed to secure write lock on update; aborting to avoid corrupting application state")
	}

	a.notifiedTxsAvailable = false
	if newPreFn != nil {
		a.preCheck = newPreFn
	}

	// TODO(berg): the use of newPostFn is very involved in the current
	//			   design that couples abci and mempool concerns. Need to
	//			   investigate possiblity of breaking that down so only
	//			   ABCI is in charge of PostCheckFuncs. Feels like a concern
	//			   that belongs to ABCI and not the individual mempool
	//			   implementations. Then again... it may make sense in the
	//			   Pool itself :thinking_face:...
	opRes, err := a.pool.OnUpdate(ctx, blockHeight, blockTxs, txResults, newPostFn)
	if err != nil {
		return err
	}
	a.applyOpResult(opRes)

	return nil
}

// TxsAvailable returns a channel which fires once for every height, and only
// when transactions are available in the mempool. It is thread-safe.
func (a *ABCI) TxsAvailable() <-chan struct{} {
	return a.txsAvailable
}

func (a *ABCI) applyOpResult(o OpResult) {
	for _, tx := range o.AddedTXs {
		a.cache.Push(tx)
	}

	if !a.cfg.KeepInvalidTxsInCache {
		for _, tx := range o.RemovedTXs {
			a.cache.Remove(tx)
		}
	}

	if o.Status == StatusTXsAvailable {
		a.notifyTxsAvailable()
	}
}

func (a *ABCI) notifyTxsAvailable() {
	if a.txsAvailable == nil || a.notifiedTxsAvailable {
		return
	}

	// channel cap is 1, so this will send once
	a.notifiedTxsAvailable = true
	select {
	case a.txsAvailable <- struct{}{}:
	default:
	}
}

func (a *ABCI) runPrecheck(tx types.Tx) error {
	if a.preCheck == nil {
		return nil
	}
	return a.preCheck(tx)
}
