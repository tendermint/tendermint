package account

import (
	"github.com/tendermint/go-ed25519"

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

func (privAccount *PrivAccount) Sign(o Signable) Signature {
	return privAccount.PrivKey.Sign(SignBytes(o))
}
