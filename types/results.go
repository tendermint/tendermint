package types

import (
	abci "github.com/tendermint/abci/types"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	"github.com/tendermint/tmlibs/merkle"
)

//-----------------------------------------------------------------------------

// ABCIResult is the deterministic component of a ResponseDeliverTx.
// TODO: add Tags
type ABCIResult struct {
	Code uint32     `json:"code"`
	Data data.Bytes `json:"data"`
}

// Hash returns the canonical hash of the ABCIResult
func (a ABCIResult) Hash() []byte {
	return wire.BinaryRipemd160(a)
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

// Bytes serializes the ABCIResponse using go-wire
func (a ABCIResults) Bytes() []byte {
	return wire.BinaryBytes(a)
}

// Hash returns a merkle hash of all results
func (a ABCIResults) Hash() []byte {
	return merkle.SimpleHashFromHashables(a.toHashables())
}

// ProveResult returns a merkle proof of one result from the set
func (a ABCIResults) ProveResult(i int) merkle.SimpleProof {
	_, proofs := merkle.SimpleProofsFromHashables(a.toHashables())
	return *proofs[i]
}

func (a ABCIResults) toHashables() []merkle.Hashable {
	l := len(a)
	hashables := make([]merkle.Hashable, l)
	for i := 0; i < l; i++ {
		hashables[i] = a[i]
	}
	return hashables
}
