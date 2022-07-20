package core

import (
	"context"

	"github.com/tendermint/tendermint/rpc/coretypes"
)

// UnsafeFlushMempool removes all transactions from the mempool.
func (env *Environment) UnsafeFlushMempool(ctx context.Context) (*coretypes.ResultUnsafeFlushMempool, error) {
	if err := env.Mempool.Flush(ctx); err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsafeFlushMempool{}, nil
}
