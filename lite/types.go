package lite

import (
	"github.com/tendermint/tendermint/types"
)

// NOTE: The Verifier interface is deprecated.  BaseVerifier can continue to
// exist, but this interface isn't very useful on its own to declare here.
//
// Verifier checks the votes to make sure the block really is signed properly.
// Verifier must know the current or recent set of validitors by some other
// means.
type Verifier interface {
	Verify(sheader types.SignedHeader) error
	ChainID() string
}
