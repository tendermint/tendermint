package ed25519

import (
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
)

const (
	PrivKeyAminoName = "tendermint/PrivKeyEd25519"
	PubKeyAminoName  = "tendermint/PubKeyEd25519"

	TypePubKeyEd25519  = "PubKeyEd25519"
	TypePrivKeyEd25519 = "PrivKeyEd25519"

	// Size of an Edwards25519 signature. Namely the size of a compressed
	// Edwards25519 point, and a field element. Both of which are 32 bytes.
	SignatureSize = 64
)

var cdc = amino.NewCodec()

func init() {
	RegisterCodec(cdc)

	crypto.RegisterKeyEncoding(crypto.KeyEncoding{
		Type:   TypePubKeyEd25519,
		Name:   PubKeyAminoName,
		Prefix: []byte{0x16, 0x24, 0xDE, 0x64},
		Length: []byte{0x20},
	})
	crypto.RegisterKeyEncoding(crypto.KeyEncoding{
		Type:   TypePrivKeyEd25519,
		Name:   PrivKeyAminoName,
		Prefix: []byte{0xA3, 0x28, 0x89, 0x10},
		Length: []byte{0x40},
	})
}

// RegisterCodec registers the Ed25519 key types with the provided amino codec.
func RegisterCodec(c *amino.Codec) {
	c.RegisterInterface((*crypto.PubKey)(nil), nil)
	c.RegisterConcrete(PubKeyEd25519{}, PubKeyAminoName, nil)

	c.RegisterInterface((*crypto.PrivKey)(nil), nil)
	c.RegisterConcrete(PrivKeyEd25519{}, PrivKeyAminoName, nil)
}
