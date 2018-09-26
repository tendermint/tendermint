package state

import (
	"github.com/tendermint/tendermint/types"
)

// TxFilter returns a function to filter transactions. The function limits the
// size of a transaction to the maximum block's data size.
func TxFilter(state State) func(tx types.Tx) bool {
	maxDataBytes := types.MaxDataBytesUnknownEvidence(
		state.ConsensusParams.BlockSize.MaxBytes,
		state.Validators.Size(),
	)
	return func(tx types.Tx) bool { return int64(len(tx)) <= maxDataBytes }
}
