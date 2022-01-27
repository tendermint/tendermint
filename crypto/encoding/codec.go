package encoding

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto/sr25519"
	"github.com/tendermint/tendermint/internal/jsontypes"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
)

func init() {
	jsontypes.MustRegister((*cryptoproto.PublicKey)(nil))
	jsontypes.MustRegister((*cryptoproto.PublicKey_Ed25519)(nil))
	jsontypes.MustRegister((*cryptoproto.PublicKey_Secp256K1)(nil))
}

// PubKeyToProto takes crypto.PubKey and transforms it to a protobuf Pubkey
func PubKeyToProto(k crypto.PubKey) (cryptoproto.PublicKey, error) {
	var kp cryptoproto.PublicKey
	switch k := k.(type) {
	case ed25519.PubKey:
		kp = cryptoproto.PublicKey{
			Sum: &cryptoproto.PublicKey_Ed25519{
				Ed25519: k,
			},
		}
	case secp256k1.PubKey:
		kp = cryptoproto.PublicKey{
			Sum: &cryptoproto.PublicKey_Secp256K1{
				Secp256K1: k,
			},
		}
	case sr25519.PubKey:
		kp = cryptoproto.PublicKey{
			Sum: &cryptoproto.PublicKey_Sr25519{
				Sr25519: k,
			},
		}
	default:
		return kp, fmt.Errorf("toproto: key type %v is not supported", k)
	}
	return kp, nil
}

// PubKeyFromProto takes a protobuf Pubkey and transforms it to a crypto.Pubkey
func PubKeyFromProto(k cryptoproto.PublicKey) (crypto.PubKey, error) {
	switch k := k.Sum.(type) {
	case *cryptoproto.PublicKey_Ed25519:
		if len(k.Ed25519) != ed25519.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeyEd25519. Got %d, expected %d",
				len(k.Ed25519), ed25519.PubKeySize)
		}
		pk := make(ed25519.PubKey, ed25519.PubKeySize)
		copy(pk, k.Ed25519)
		return pk, nil
	case *cryptoproto.PublicKey_Secp256K1:
		if len(k.Secp256K1) != secp256k1.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeySecp256k1. Got %d, expected %d",
				len(k.Secp256K1), secp256k1.PubKeySize)
		}
		pk := make(secp256k1.PubKey, secp256k1.PubKeySize)
		copy(pk, k.Secp256K1)
		return pk, nil
	case *cryptoproto.PublicKey_Sr25519:
		if len(k.Sr25519) != sr25519.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeySr25519. Got %d, expected %d",
				len(k.Sr25519), sr25519.PubKeySize)
		}
		pk := make(sr25519.PubKey, sr25519.PubKeySize)
		copy(pk, k.Sr25519)
		return pk, nil
	default:
		return nil, fmt.Errorf("fromproto: key type %v is not supported", k)
	}
}
