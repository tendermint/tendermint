package interfaces

import (
	"time"

	"github.com/tendermint/tendermint/crypto"
)

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() int64                                     // height of the equivocation
	Time() time.Time                                   // time of the equivocation
	Address() []byte                                   // address of the equivocating validator
	Bytes() []byte                                     // bytes which comprise the evidence
	Hash() []byte                                      // hash of the evidence
	Verify(chainID string, pubKey crypto.PubKey) error // verify the evidence
	Equals(Evidence) bool                              // check equality of evidence
	ValidateBasic() error
	String() string
}
