package types

import (
	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-merkle"
)

type Tx []byte

// NOTE: this is the hash of the go-wire encoded Tx.
// Tx has no types at this level, so just length-prefixed.
// Alternatively, it may make sense to add types here and let
// []byte be type 0x1 so we can have versioned txs if need be in the future.
func (tx Tx) Hash() []byte {
	return merkle.SimpleHashFromBinary(tx)
}

type Txs []Tx

func (txs Txs) Hash() []byte {
	// Recursive impl.
	// Copied from go-merkle to avoid allocations
	switch len(txs) {
	case 0:
		return nil
	case 1:
		return txs[0].Hash()
	default:
		left := Txs(txs[:(len(txs)+1)/2]).Hash()
		right := Txs(txs[(len(txs)+1)/2:]).Hash()
		return merkle.SimpleHashFromTwoHashes(left, right)
	}
}

// TxResult contains results of executing the transaction.
//
// One usage is indexing transaction results.
type TxResult struct {
	Height    uint64                 `json:"height"`
	Index     uint32                 `json:"index"`
	DeliverTx abci.ResponseDeliverTx `json:"deliver_tx"`
}
