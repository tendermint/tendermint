package types

import (
	abci "github.com/tendermint/abci/types"
)

// TxResult contains results of executing the transaction.
//
// One usage is indexing transaction results.
type TxResult struct {
	Height            uint64
	Index             uint32
	DeliverTxResponse abci.ResponseDeliverTx
}
