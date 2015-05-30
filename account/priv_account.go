package account

import (
	"github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

type PrivAccount struct {
	Address []byte  `json:"address"`
	PubKey  PubKey  `json:"pub_key"`
	PrivKey PrivKey `json:"priv_key"`
}

// Generates a new account with private key.
func GenPrivAccount() *PrivAccount {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], CRandBytes(32))
	pubKeyBytes := ed25519.MakePublicKey(privKeyBytes)
	pubKey := PubKeyEd25519(pubKeyBytes[:])
	privKey := PrivKeyEd25519(privKeyBytes[:])
	return &PrivAccount{
		Address: pubKey.Address(),
		PubKey:  pubKey,
		PrivKey: privKey,
	}
}

// Generates a new account with private key from SHA256 hash of a secret
func GenPrivAccountFromSecret(secret []byte) *PrivAccount {
	privKey32 := binary.BinarySha256(secret)
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], privKey32)
	pubKeyBytes := ed25519.MakePublicKey(privKeyBytes)
	pubKey := PubKeyEd25519(pubKeyBytes[:])
	privKey := PrivKeyEd25519(privKeyBytes[:])
	return &PrivAccount{
		Address: pubKey.Address(),
		PubKey:  pubKey,
		PrivKey: privKey,
	}
}

func GenPrivAccountFromKey(privKeyBytes [64]byte) *PrivAccount {
	pubKeyBytes := ed25519.MakePublicKey(&privKeyBytes)
	pubKey := PubKeyEd25519(pubKeyBytes[:])
	privKey := PrivKeyEd25519(privKeyBytes[:])
	return &PrivAccount{
		Address: pubKey.Address(),
		PubKey:  pubKey,
		PrivKey: privKey,
	}
}

func (privAccount *PrivAccount) Sign(chainID string, o Signable) Signature {
	return privAccount.PrivKey.Sign(SignBytes(chainID, o))
}

func (privAccount *PrivAccount) String() string {
	return Fmt("PrivAccount{%X}", privAccount.Address)
}
