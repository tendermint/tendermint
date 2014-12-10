package wallet

import (
	"github.com/tendermint/go-ed25519"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
)

type PrivAccount struct {
	PubKey  PubKey
	PrivKey PrivKey
}

// Generates a new account with private key.
func GenPrivAccount() *PrivAccount {
	privKey := CRandBytes(32)
	pubKey := ed25519.MakePubKey(privKey)
	return &PrivAccount{
		PubKeyEd25519{
			PubKey: pubKey,
		},
		PrivKeyEd25519{
			PubKey:  pubKey,
			PrivKey: privKey,
		},
	}
}
