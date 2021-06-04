package core

import (
	"fmt"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

// UnsafeFlushMempool removes all transactions from the mempool.
func (env *Environment) UnsafeFlushMempool(ctx *rpctypes.Context) (*ctypes.ResultUnsafeFlushMempool, error) {
	env.Mempool.Flush()
	return &ctypes.ResultUnsafeFlushMempool{}, nil
}

// UnsafeReIndex re-index the block/transaction events into the eventsinks.
func (env *Environment) UnsafeReIndex(
	ctx *rpctypes.Context,
	start int64,
	end int64) (*ctypes.ResultUnsafeReIndex, error) {

	base := env.BlockStore.Base()

	if start <= base {
		return nil, fmt.Errorf("%s (requested start height: %d, base height: %d)", ctypes.ErrHeightNotAvailable, start, base)
	}

	height := env.BlockStore.Height()
	if start > height {
		return nil, fmt.Errorf(
			"%s (requested start height: %d, store height: %d)", ctypes.ErrHeightNotAvailable, start, height)
	}

	if end <= base {
		return nil, fmt.Errorf(
			"%s (requested end height: %d, base height: %d)", ctypes.ErrHeightNotAvailable, end, base)
	}

	if end < start {
		return nil, fmt.Errorf(
			"%s (requested the end height: %d is less than the start height: %d)", ctypes.ErrInvalidRequest, start, end)
	}

	if end > height {
		end = height
	}

	if !indexer.IndexingEnabled(env.EventSinks) {
		return nil, fmt.Errorf("no event sink has been enabled")
	}

	for i := start; i <= end; i++ {
		b := env.BlockStore.LoadBlock(i)
		if b == nil {
			return nil, fmt.Errorf("not able to load block at height %d from the blockstore", i)
		}

		r, err := env.StateStore.LoadABCIResponses(i)
		if err != nil {
			return nil, fmt.Errorf("not able to load ABCI Response at height %d from the statestore", i)
		}

		e := types.EventDataNewBlockHeader{
			Header:           b.Header,
			NumTxs:           int64(len(b.Txs)),
			ResultBeginBlock: *r.BeginBlock,
			ResultEndBlock:   *r.EndBlock,
		}

		var batch *indexer.Batch
		if e.NumTxs > 0 {
			batch = indexer.NewBatch(e.NumTxs)

			for i, tx := range b.Data.Txs {
				tr := abcitypes.TxResult{
					Height: b.Height,
					Index:  uint32(i),
					Tx:     tx,
					Result: *(r.DeliverTxs[i]),
				}

				_ = batch.Add(&tr)
			}
		}

		for _, sink := range env.EventSinks {
			if err := sink.IndexBlockEvents(e); err != nil {
				return nil, err
			}

			if batch != nil {
				if err := sink.IndexTxEvents(batch.Ops); err != nil {
					return nil, err
				}
			}
		}

	}

	return &ctypes.ResultUnsafeReIndex{Result: "re-index finished"}, nil
}
