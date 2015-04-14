package account

import (
	"fmt"

	"github.com/tendermint/tendermint/binary"
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
var _ = binary.RegisterInterface(
	struct{ Signature }{},
	binary.ConcreteType{SignatureEd25519{}, SignatureTypeEd25519},
)

//-------------------------------------

// Implements Signature
type SignatureEd25519 []byte

func (sig SignatureEd25519) IsNil() bool { return false }

func (sig SignatureEd25519) IsZero() bool { return len(sig) == 0 }

func (sig SignatureEd25519) String() string { return fmt.Sprintf("%X", Fingerprint(sig)) }
