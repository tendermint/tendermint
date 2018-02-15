package types

import (
	abci "github.com/tendermint/abci/types"
	wire "github.com/tendermint/tendermint/wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/merkle"
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
	return tmHash(a)
}

// ABCIResults wraps the deliver tx results to return a proof
type ABCIResults []ABCIResult

// NewResults creates ABCIResults from ResponseDeliverTx
func NewResults(del []*abci.ResponseDeliverTx) ABCIResults {
	res := make(ABCIResults, len(del))
	for i, d := range del {
		res[i] = NewResultFromResponse(d)
	}
	return res
}

func NewResultFromResponse(response *abci.ResponseDeliverTx) ABCIResult {
	return ABCIResult{
		Code: response.Code,
		Data: response.Data,
	}
}

// Bytes serializes the ABCIResponse using wire
func (a ABCIResults) Bytes() []byte {
	bz, err := wire.MarshalBinary(a)
	if err != nil {
		panic(err)
	}
	return bz
}

// Hash returns a merkle hash of all results
func (a ABCIResults) Hash() []byte {
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
