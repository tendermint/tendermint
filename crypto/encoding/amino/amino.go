package cryptoAmino

import (
	"reflect"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/multisig"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

var cdc = amino.NewCodec()

// routeTable is used to map public key concrete types back
// to their amino routes. This should eventually be handled
// by amino. Example usage:
// routeTable[reflect.TypeOf(ed25519.PubKeyEd25519{})] = ed25519.PubKeyAminoRoute
var routeTable = make(map[reflect.Type]string, 3)

func init() {
	// NOTE: It's important that there be no conflicts here,
	// as that would change the canonical representations,
	// and therefore change the address.
	// TODO: Remove above note when
	// https://github.com/tendermint/go-amino/issues/9
	// is resolved
	RegisterAmino(cdc)

	// TODO: Have amino provide a way to go from concrete struct to route directly.
	// Its currently a private API
	routeTable[reflect.TypeOf(ed25519.PubKeyEd25519{})] = ed25519.PubKeyAminoRoute
	routeTable[reflect.TypeOf(secp256k1.PubKeySecp256k1{})] = secp256k1.PubKeyAminoRoute
	routeTable[reflect.TypeOf(&multisig.PubKeyMultisigThreshold{})] = multisig.PubKeyMultisigThresholdAminoRoute
}

// PubkeyAminoRoute returns the amino route of a pubkey
// cdc is currently passed in, as eventually this will not be using
// a package level codec.
func PubkeyAminoRoute(cdc *amino.Codec, key crypto.PubKey) (string, bool) {
	route, found := routeTable[reflect.TypeOf(key)]
	return route, found
}

// RegisterAmino registers all crypto related types in the given (amino) codec.
func RegisterAmino(cdc *amino.Codec) {
	// These are all written here instead of
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(ed25519.PubKeyEd25519{},
		ed25519.PubKeyAminoRoute, nil)
	cdc.RegisterConcrete(secp256k1.PubKeySecp256k1{},
		secp256k1.PubKeyAminoRoute, nil)
	cdc.RegisterConcrete(multisig.PubKeyMultisigThreshold{},
		multisig.PubKeyMultisigThresholdAminoRoute, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(ed25519.PrivKeyEd25519{},
		ed25519.PrivKeyAminoRoute, nil)
	cdc.RegisterConcrete(secp256k1.PrivKeySecp256k1{},
		secp256k1.PrivKeyAminoRoute, nil)
}

func PrivKeyFromBytes(privKeyBytes []byte) (privKey crypto.PrivKey, err error) {
	err = cdc.UnmarshalBinaryBare(privKeyBytes, &privKey)
	return
}

func PubKeyFromBytes(pubKeyBytes []byte) (pubKey crypto.PubKey, err error) {
	err = cdc.UnmarshalBinaryBare(pubKeyBytes, &pubKey)
	return
}
