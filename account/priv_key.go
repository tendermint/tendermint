package account

import (
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/ed25519/extra25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// PrivKey is part of PrivAccount and state.PrivValidator.
type PrivKey interface {
	Sign(msg []byte) Signature
	PubKey() PubKey
}

// Types of PrivKey implementations
const (
	PrivKeyTypeEd25519 = byte(0x01)
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ PrivKey }{},
	binary.ConcreteType{PrivKeyEd25519{}, PrivKeyTypeEd25519},
)

//-------------------------------------

// Implements PrivKey
type PrivKeyEd25519 []byte

func (privKey PrivKeyEd25519) Sign(msg []byte) Signature {
	pubKey := privKey.PubKey().(PubKeyEd25519)
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], privKey[:])
	copy(privKeyBytes[32:], pubKey[:])
	signatureBytes := ed25519.Sign(privKeyBytes, msg)
	return SignatureEd25519(signatureBytes[:])
}

func (privKey PrivKeyEd25519) PubKey() PubKey {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:], privKey[:])
	return PubKeyEd25519(ed25519.MakePublicKey(privKeyBytes)[:])
}

func (privKey PrivKeyEd25519) ToCurve25519() *[32]byte {
	keyEd25519, keyCurve25519 := new([64]byte), new([32]byte)
	copy(keyEd25519[:], privKey)
	extra25519.PrivateKeyToCurve25519(keyCurve25519, keyEd25519)
	return keyCurve25519
}

func (privKey PrivKeyEd25519) String() string {
	return Fmt("PrivKeyEd25519{*****}")
}

func GenPrivKeyEd25519() PrivKeyEd25519 {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], CRandBytes(32))
	ed25519.MakePublicKey(privKeyBytes)
	return PrivKeyEd25519(privKeyBytes[:])
}
