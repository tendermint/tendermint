package core

import (
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// UnsafeFlushMempool removes all transactions from the mempool.
func (env *Environment) UnsafeFlushMempool(ctx *rpctypes.Context) (*coretypes.ResultUnsafeFlushMempool, error) {
	env.Mempool.Flush()
	return &coretypes.ResultUnsafeFlushMempool{}, nil
}
