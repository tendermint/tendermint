package state

import (
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

// TxPreCheck returns a function to filter transactions before processing.
// The function limits the size of a transaction to the block's maximum data size.
func TxPreCheck(state State) mempl.PreCheckFunc {
	maxDataBytes := types.MaxDataBytesUnknownEvidence(
		state.ConsensusParams.Block.MaxBytes,
		state.Validators.Size(),
	)
	return mempl.PreCheckAminoMaxBytes(maxDataBytes)
}

// TxPostCheck returns a function to filter transactions after processing.
// The function limits the gas wanted by a transaction to the block's maximum total gas.
func TxPostCheck(state State) mempl.PostCheckFunc {
	return mempl.PostCheckMaxGas(state.ConsensusParams.Block.MaxGas)
}
