package multisig

import (
	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto/sr25519"
)

// TODO: Figure out API for others to either add their own pubkey types, or
// to make verify / marshal accept a cdc.
const (
	PubKeyRoute = "tendermint/PubKeyMultisigThreshold"
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKey{},
		PubKeyRoute, nil)
	cdc.RegisterConcrete(ed25519.PubKey{},
		ed25519.PubKeyName, nil)
	cdc.RegisterConcrete(sr25519.PubKey{},
		sr25519.PubKeyName, nil)
	cdc.RegisterConcrete(secp256k1.PubKey{},
		secp256k1.PubKeyName, nil)
}
