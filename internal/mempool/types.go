package mempool

import (
	"context"
	"fmt"
	"math"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	MempoolChannel = p2p.ChannelID(0x30)

	// PeerCatchupSleepIntervalMS defines how much time to sleep if a peer is behind
	PeerCatchupSleepIntervalMS = 100

	// UnknownPeerID is the peer ID to use when running CheckTx when there is
	// no peer (e.g. RPC)
	UnknownPeerID uint16 = 0

	MaxActiveIDs = math.MaxUint16
)

// Status is the status of the Pool.
type Status string

const (
	// StatusTXsAvailable indicates the pool has txs available.
	StatusTXsAvailable = "txs_available"
)

type MempoolABCI interface {
	// CheckTx executes a new transaction against the application to determine
	// its validity and whether it should be added to the mempool.
	CheckTx(ctx context.Context, tx types.Tx, callback func(*abci.ResponseCheckTx), txInfo TxInfo) error

	// EnableTxsAvailable initializes the TxsAvailable channel, ensuring it will
	// trigger once every height when transactions are available.
	EnableTxsAvailable()

	// Flush will clear all caches in the MempoolABCI and clear any volatile
	// memory within the pool.
	Flush(ctx context.Context) error

	// PoolMeta returns the metadata for the underlying pool implementation.
	PoolMeta() PoolMeta

	// PrepBlockFinality prepares the mempool for finalizing a block. This may require
	// locking or other concerns that must be handled before a block is safe to commit.
	PrepBlockFinality(ctx context.Context) (finishFn func(), err error)

	// Reap returns Txs from the given pool. It is up to the pool implementation to define
	// how they handle the possible predicates from option combinations.
	Reap(ctx context.Context, opts ...ReapOptFn) (types.Txs, error)

	// Remove removes Txs from the given pool. It is up to the pool implementation to
	// define how they handle the possible predicates from option combinations.
	Remove(ctx context.Context, opts ...RemOptFn) error

	// TxsAvailable returns a channel which fires once for every height, and only
	// when transactions are available in the mempool.
	//
	// NOTE:
	// 1. The returned channel may be nil if EnableTxsAvailable was not called.
	TxsAvailable() <-chan struct{}

	// Update informs the mempool that the given txs were committed and can be
	// discarded.
	//
	// NOTE:
	// 1. This should be called *after* block is committed by consensus.
	// 2. PrepBlockFinality must be called before calling Update, effectively
	//	  preparing the MempoolABCI -> App communication so that the application
	//	  state is correct.
	Update(
		ctx context.Context,
		blockHeight int64,
		blockTxs types.Txs,
		txResults []*abci.ExecTxResult,
		newPreFn PreCheckFunc,
		newPostFn PostCheckFunc,
	) error
}

// OpResult is the result of a pool operation. This result informs the ABCI type
// what happened in the storage/pool layer so it can maintain cache coherence.
type OpResult struct {
	AddedTXs   types.Txs
	RemovedTXs types.Txs
	Status     Status
}

// PoolMeta is the metadata for a given pool.
type PoolMeta struct {
	// Type describes the type of mempool store. Examples could be priority or narwhal.
	Type string
	// Size is the num of TXs or whatever the unit of measure for the store.
	Size int
	// TotalBytes is a measure of hte store's data size.
	TotalBytes int64
}

// Pool defines the underlying pool storage engine behavior.
type Pool interface {
	// CheckTXCallback is called by the ABCI type during CheckTX. When called by ABCI,
	// the CheckTXCallback can assume the ABCI type will hold the app write lock.
	CheckTXCallback(ctx context.Context, tx types.Tx, res *abci.ResponseCheckTx, txInfo TxInfo) (OpResult, error)

	// Flush removes all transactions from the mempool and caches.
	Flush(ctx context.Context) error

	// Meta returns metadata for the pool.
	Meta() PoolMeta

	// OnUpdate is called by the ABCI on an Update. The ABCI type requires the caller
	// to have called PrepBlockFinality before issuing an Update, thus forcing the
	// caller to acquire the lock before Update and subsequently, this OnUpdate call
	// is made.
	OnUpdate(
		ctx context.Context,
		blockHeight int64,
		blockTxs types.Txs,
		txResults []*abci.ExecTxResult,
		newPostFn PostCheckFunc,
	) (OpResult, error)

	// Reap returns Txs from the given pool. It is up to the pool implementation to define
	// how they handle the possible predicates from option combinations.
	Reap(ctx context.Context, opts ...ReapOptFn) (types.Txs, error)

	// Remove removes TXs by the provided RemOptFn. If an argument is provided to the
	// options that don't make sense for the given pool, then it will be ignored.
	Remove(ctx context.Context, opts ...RemOptFn) (OpResult, error)
}

// DisableReapOpt sets the reap opt to disabled. This is the default value for all
// fields in the ReapOption type. If you call Reap(ctx), you will get all TXs within
// the mempool (if allowed).
const DisableReapOpt = -1

// ReapOption is the options to reaping a collection useful to proposal from the mempool.
// A collection for proposal in teh Clist or Priority mempools, would be a list of TXs.
// For a DAG (narwhal) mempool, we'd want the DAGCerts for the proposal. However,
// we use the same predicates to reap from all mempools. When a predicate has multiple opts
// specified, will take the collections that satisfy all limits specified are adhered too.
// When a field is set to DisablePredicate (-1), the field will not be enforced.
//
// Example predicate: BlockSizeLimit AND NumTXs are both set enforcing that BlockSizeLimit
//					  and NumTXs limits are satisfied in the Reaping.
type ReapOption struct {
	BlockSizeLimit int64
	GasLimit       int64
	NumTXs         int
}

// CoalesceReapOpts provides a quick way to coalesce ReapOptFn's with default field
// values set to DisableReapOpt.
func CoalesceReapOpts(opts ...ReapOptFn) ReapOption {
	opt := ReapOption{
		BlockSizeLimit: DisableReapOpt,
		GasLimit:       DisableReapOpt,
		NumTXs:         DisableReapOpt,
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

// ReapOptFn is a functional option for setting the reap predicates.
type ReapOptFn func(*ReapOption)

// ReapBytes will limit the reap by a maximum number of bytes. Note, if
// you provide a value less than 0, it will ignore the max bytes limit.
// This is the same as if you did not provide the option to the Reap method.
func ReapBytes(maxBytes int64) ReapOptFn {
	return func(option *ReapOption) {
		option.BlockSizeLimit = maxBytes
	}
}

// ReapGas will limit the reap by the maxGas. Note, if you provide a
// value less than 0, it will ignore the gas limit. This is the same as if
// you did not provide the option to the Reap method.
func ReapGas(maxGas int64) ReapOptFn {
	return func(option *ReapOption) {
		option.GasLimit = maxGas
	}
}

// ReapTXs will limit the reap to a number of TXs. Note, if you provide
// a value less than 0, it will ignore the TXs. This is the same as if
// you did not provide the option to the Reap method.
func ReapTXs(maxTXs int) ReapOptFn {
	return func(option *ReapOption) {
		option.NumTXs = maxTXs
	}
}

// RemOption is an option for removing TXs from a pool.
type RemOption struct {
	TXKeys []types.TxKey
}

// RemOptFn is a functional option definition for setting fields on RemOption.
type RemOptFn func(option *RemOption)

// CoalesceRemOptFns returns a RemOption ready for processing.
func CoalesceRemOptFns(opts ...RemOptFn) RemOption {
	var opt RemOption
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

// RemByTXKeys removes a transaction(s), identified by its key, from the mempool.
func RemByTXKeys(txs ...types.TxKey) RemOptFn {
	return func(option *RemOption) {
		option.TXKeys = append(option.TXKeys, txs...)
	}
}

// PreCheckFunc is an optional filter executed before CheckTx and rejects
// transaction if false is returned. An example would be to ensure that a
// transaction doesn't exceeded the block size.
type PreCheckFunc func(types.Tx) error

// PostCheckFunc is an optional filter executed after CheckTx and rejects
// transaction if false is returned. An example would be to ensure a
// transaction doesn't require more gas than available for the block.
type PostCheckFunc func(types.Tx, *abci.ResponseCheckTx) error

// PreCheckMaxBytes checks that the size of the transaction is smaller or equal
// to the expected maxBytes.
func PreCheckMaxBytes(maxBytes int64) PreCheckFunc {
	return func(tx types.Tx) error {
		txSize := types.ComputeProtoSizeForTxs([]types.Tx{tx})

		if txSize > maxBytes {
			return fmt.Errorf("tx size is too big: %d, max: %d", txSize, maxBytes)
		}

		return nil
	}
}

// PostCheckMaxGas checks that the wanted gas is smaller or equal to the passed
// maxGas. Returns nil if maxGas is -1.
func PostCheckMaxGas(maxGas int64) PostCheckFunc {
	return func(tx types.Tx, res *abci.ResponseCheckTx) error {
		if maxGas == -1 {
			return nil
		}
		if res.GasWanted < 0 {
			return fmt.Errorf("gas wanted %d is negative",
				res.GasWanted)
		}
		if res.GasWanted > maxGas {
			return fmt.Errorf("gas wanted %d is greater than max gas %d",
				res.GasWanted, maxGas)
		}

		return nil
	}
}
