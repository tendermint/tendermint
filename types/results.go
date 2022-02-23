package types

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
)

// ABCIResults wraps the deliver tx results to return a proof.
type ABCIResults []*abci.ExecTxResult

// NewResults strips non-deterministic fields from ResponseDeliverTx responses
// and returns ABCIResults.
func NewResults(responses []*abci.ExecTxResult) ABCIResults {
	res := make(ABCIResults, len(responses))
	for i, d := range responses {
		res[i] = deterministicResponseDeliverTx(d)
	}
	return res
}

// Hash returns a merkle hash of all results.
func (a ABCIResults) Hash() []byte {
	return merkle.HashFromByteSlices(a.toByteSlices())
}

// ProveResult returns a merkle proof of one result from the set
func (a ABCIResults) ProveResult(i int) merkle.Proof {
	_, proofs := merkle.ProofsFromByteSlices(a.toByteSlices())
	return *proofs[i]
}

func (a ABCIResults) toByteSlices() [][]byte {
	l := len(a)
	bzs := make([][]byte, l)
	for i := 0; i < l; i++ {
		bz, err := a[i].Marshal()
		if err != nil {
			panic(err)
		}
		bzs[i] = bz
	}
	return bzs
}

// deterministicResponseDeliverTx strips non-deterministic fields from
// ResponseDeliverTx and returns another ResponseDeliverTx.
func deterministicResponseDeliverTx(response *abci.ExecTxResult) *abci.ExecTxResult {
	return &abci.ExecTxResult{
		Code:      response.Code,
		Data:      response.Data,
		GasWanted: response.GasWanted,
		GasUsed:   response.GasUsed,
	}
}
