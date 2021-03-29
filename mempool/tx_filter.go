package mempool

import (
	"github.com/tendermint/tendermint/types"
)

// TxPreCheck returns a function to filter transactions before processing.
// The function limits the size of a transaction to the block's maximum data size.
func (mem *CListMempool) TxPreCheck(state SmState) PreCheckFunc {
	maxDataBytes := types.MaxDataBytesNoEvidence(
		state.GetBlockMaxBytes(),
		state.GetValidatorSize(),
	)
	return PreCheckMaxBytes(maxDataBytes)
}

// TxPostCheck returns a function to filter transactions after processing.
// The function limits the gas wanted by a transaction to the block's maximum total gas.
func (mem *CListMempool) TxPostCheck(state SmState) PostCheckFunc {
	return PostCheckMaxGas(state.GetBlockMaxGas())
}
