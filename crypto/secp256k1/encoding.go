package secp256k1

import (
	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/crypto"
	tmjson "github.com/tendermint/tendermint/libs/json"
)

const (
	PrivKeyAminoName = "tendermint/PrivKeySecp256k1"
	PubKeyAminoName  = "tendermint/PubKeySecp256k1"
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKey{},
		PubKeyAminoName, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKey{},
		PrivKeyAminoName, nil)

	tmjson.RegisterType(PubKey{}, PubKeyAminoName)
	tmjson.RegisterType(PrivKey{}, PrivKeyAminoName)
}
