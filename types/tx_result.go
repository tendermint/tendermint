package types

import (
	abci "github.com/tendermint/abci/types"
)

// TxResult gets stored into `BlockStore`, so we could get
// information about the transaction and send it to the caller.
//
// Empty (nil) response and 0 height mean transaction was accepted,
// but neither checked nor delivered.
type TxResult struct {
	Tx                Tx
	Height            int
	DeliverTxResponse abci.ResponseDeliverTx
}
