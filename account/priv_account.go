package account

import (
	. "github.com/tendermint/tendermint/common"
)

type PrivAccount struct {
	Address []byte
	PubKey  PubKey
	PrivKey PrivKey
}

// Generates a new account with private key.
func GenPrivAccount() *PrivAccount {
	privKey := PrivKeyEd25519(CRandBytes(32))
	return &PrivAccount{
		Address: privKey.PubKey().Address(),
		PubKey:  privKey.PubKey(),
		PrivKey: privKey,
	}
}

func (privAccount *PrivAccount) Sign(o Signable) Signature {
	return privAccount.PrivKey.Sign(SignBytes(o))
}
