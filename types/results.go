package types

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	cmn "github.com/tendermint/tendermint/libs/common"
)

//-----------------------------------------------------------------------------

// ABCIResult is the deterministic component of a ResponseDeliverTx.
// TODO: add tags and other fields
// https://github.com/tendermint/tendermint/issues/1007
type ABCIResult struct {
	Code uint32       `json:"code"`
	Data cmn.HexBytes `json:"data"`
}

// Bytes returns the amino encoded ABCIResult
func (a ABCIResult) Bytes() []byte {
	return cdcEncode(a)
}

// ABCIResults wraps the deliver tx results to return a proof
type ABCIResults []ABCIResult

// NewResults creates ABCIResults from the list of ResponseDeliverTx.
func NewResults(responses []*abci.ResponseDeliverTx) ABCIResults {
	res := make(ABCIResults, len(responses))
	for i, d := range responses {
		res[i] = NewResultFromResponse(d)
	}
	return res
}

// NewResultFromResponse creates ABCIResult from ResponseDeliverTx.
func NewResultFromResponse(response *abci.ResponseDeliverTx) ABCIResult {
	return ABCIResult{
		Code: response.Code,
		Data: response.Data,
	}
}

// Bytes serializes the ABCIResponse using amino
func (a ABCIResults) Bytes() []byte {
	bz, err := cdc.MarshalBinaryLengthPrefixed(a)
	if err != nil {
		panic(err)
	}
	return bz
}

// Hash returns a merkle hash of all results
func (a ABCIResults) Hash() []byte {
	// NOTE: we copy the impl of the merkle tree for txs -
	// we should be consistent and either do it for both or not.
	return merkle.SimpleHashFromByteSlices(a.toByteSlices())
}

// ProveResult returns a merkle proof of one result from the set
func (a ABCIResults) ProveResult(i int) merkle.SimpleProof {
	_, proofs := merkle.SimpleProofsFromByteSlices(a.toByteSlices())
	return *proofs[i]
}

func (a ABCIResults) toByteSlices() [][]byte {
	l := len(a)
	bzs := make([][]byte, l)
	for i := 0; i < l; i++ {
		bzs[i] = a[i].Bytes()
	}
	return bzs
}
