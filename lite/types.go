package lite

import (
	"github.com/tendermint/tendermint/types"
)

// Certifier checks the votes to make sure the block really is signed properly.
// Certifier must know the current or recent set of validitors by some other
// means.
type Certifier interface {
	Certify(sheader types.SignedHeader) error
	ChainID() string
}
