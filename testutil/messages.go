package testutil

import (
	"github.com/tendermint/abci/types"
	"github.com/tendermint/go-crypto"
)

//----------------------------------------

// UTILITY
func Validator(secret string, power uint64) *types.Validator {
	privKey := crypto.GenPrivKeyEd25519FromSecret([]byte(secret))
	return &types.Validator{
		PubKey: privKey.PubKey().Bytes(),
		Power:  power,
	}
}
