package encoding

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	pc "github.com/tendermint/tendermint/proto/crypto"
)

// PubKeyToProto takes crypto.PubKey and transforms it to a protobuf Pubkey
func PubKeyToProto(k crypto.PubKey) (pc.PublicKey, error) {
	var kp pc.PublicKey
	switch k := k.(type) {
	case *ed25519.PubKey:
		kp = pc.PublicKey{
			Sum: &pc.PublicKey_Ed25519{
				Ed25519: *k,
			},
		}
	default:
		fmt.Println(k)
		return kp, errors.New("key type is not supported")
	}
	return kp, nil
}

// PubKeyFromProto takes a protobuf Pubkey and transforms it to a crypto.Pubkey
func PubKeyFromProto(k pc.PublicKey) (crypto.PubKey, error) {
	// var cp crypto.PubKey
	switch k.Sum.(type) {
	case *pc.PublicKey_Ed25519:
		cp := ed25519.PubKey(k.GetEd25519())

		return cp, nil
	default:
		return nil, errors.New("key type not supported")
	}
	// return cp, nil
}

// PrivKeyToProto takes crypto.PrivKey and transforms it to a protobuf PrivKey
func PrivKeyToProto(k crypto.PrivKey) (pc.PrivateKey, error) {
	var kp pc.PrivateKey
	switch k := k.(type) {
	case *ed25519.PrivKey:
		kp = pc.PrivateKey{
			Sum: &pc.PrivateKey_Ed25519{
				Ed25519: *k,
			},
		}
	default:
		return kp, errors.New("key type is not supported")
	}
	return kp, nil
}

// PrivKeyFromProto takes a protobuf PrivateKey and transforms it to a crypto.PrivKey
func PrivKeyFromProto(k pc.PrivateKey) (crypto.PrivKey, error) {
	var cp crypto.PrivKey
	switch k := k.Sum.(type) {
	case *pc.PrivateKey_Ed25519:
		cp = ed25519.PrivKey(k.Ed25519)
	default:
		return nil, errors.New("key type not supported")
	}
	return cp, nil
}
