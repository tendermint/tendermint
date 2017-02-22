package types

import (
	abci "github.com/tendermint/abci/types"
)

// TxResult contains results of executing the transaction.
//
// One usage is indexing transaction results.
type TxResult struct {
	Height    uint64                 `json:"height"`
	Index     uint32                 `json:"index"`
	DeliverTx abci.ResponseDeliverTx `json:"deliver_tx"`
}
