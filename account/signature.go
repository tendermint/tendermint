package account

import (
	"errors"
	"fmt"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// Signature is a part of Txs and consensus Votes.
type Signature interface {
}

// Types of Signature implementations
const (
	SignatureTypeEd25519 = byte(0x01)
)

// for binary.readReflect
var _ = RegisterInterface(
	struct{ Signature }{},
	ConcreteType{SignatureEd25519{}},
)

//-------------------------------------

// Implements Signature
type SignatureEd25519 []byte

func (sig SignatureEd25519) TypeByte() byte { return SignatureTypeEd25519 }

func (sig SignatureEd25519) ValidateBasic() error {
	if len(sig) != ed25519.SignatureSize {
		return errors.New("Invalid SignatureEd25519 signature size")
	}
	return nil
}

func (sig SignatureEd25519) IsZero() bool {
	return len(sig) == 0
}

func (sig SignatureEd25519) String() string {
	return fmt.Sprintf("%X", Fingerprint(sig))
}
