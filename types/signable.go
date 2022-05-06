package types

import (
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmmath "github.com/tendermint/tendermint/libs/math"
)

// MaxSignatureSize is a maximum allowed signature size for the Proposal
// and Vote.
// XXX: secp256k1 does not have Size nor MaxSize defined.
var MaxSignatureSize = tmmath.MaxInt(ed25519.SignatureSize, 64)
