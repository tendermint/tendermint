package types

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	cmn "github.com/tendermint/tendermint/libs/common"
)

//-----------------------------------------------------------------------------

// ABCIResult is the deterministic component of a ResponseDeliverTx.
// TODO: add Tags
type ABCIResult struct {
	Code uint32       `json:"code"`
	Data cmn.HexBytes `json:"data"`
}

// Hash returns the canonical hash of the ABCIResult
func (a ABCIResult) Hash() []byte {
	bz := aminoHash(a)
	return bz
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

// Bytes serializes the ABCIResponse using wire
func (a ABCIResults) Bytes() []byte {
	bz, err := cdc.MarshalBinary(a)
	if err != nil {
		panic(err)
	}
	return bz
}

// Hash returns a merkle hash of all results
func (a ABCIResults) Hash() []byte {
	// NOTE: we copy the impl of the merkle tree for txs -
	// we should be consistent and either do it for both or not.
	return merkle.SimpleHashFromHashers(a.toHashers())
}

// ProveResult returns a merkle proof of one result from the set
func (a ABCIResults) ProveResult(i int) merkle.SimpleProof {
	_, proofs := merkle.SimpleProofsFromHashers(a.toHashers())
	return *proofs[i]
}

func (a ABCIResults) toHashers() []merkle.Hasher {
	l := len(a)
	hashers := make([]merkle.Hasher, l)
	for i := 0; i < l; i++ {
		hashers[i] = a[i]
	}
	return hashers
}
