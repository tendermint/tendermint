package account

import (
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

type PrivAccount struct {
	Address []byte  `json:"address"`
	PubKey  PubKey  `json:"pub_key"`
	PrivKey PrivKey `json:"priv_key"`
}

func (pA *PrivAccount) Generate(index int) *PrivAccount {
	newPrivKey := pA.PrivKey.(PrivKeyEd25519).Generate(index)
	newPubKey := newPrivKey.PubKey()
	newAddress := newPubKey.Address()
	return &PrivAccount{
		Address: newAddress,
		PubKey:  newPubKey,
		PrivKey: newPrivKey,
	}
}

func (pA *PrivAccount) Sign(chainID string, o Signable) Signature {
	return pA.PrivKey.Sign(SignBytes(chainID, o))
}

func (pA *PrivAccount) String() string {
	return Fmt("PrivAccount{%X}", pA.Address)
}

//----------------------------------------

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
	privKey32 := binary.BinarySha256(secret) // Not Ripemd160 because we want 32 bytes.
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

func GenPrivAccountFromPrivKeyBytes(privKeyBytes []byte) *PrivAccount {
	if len(privKeyBytes) != 64 {
		panic(Fmt("Expected 64 bytes but got %v", len(privKeyBytes)))
	}
	privKeyBytes64 := [64]byte{}
	copy(privKeyBytes64[:], privKeyBytes)
	pubKeyBytes := ed25519.MakePublicKey(&privKeyBytes64)
	pubKey := PubKeyEd25519(pubKeyBytes[:])
	privKey := PrivKeyEd25519(privKeyBytes)
	return &PrivAccount{
		Address: pubKey.Address(),
		PubKey:  pubKey,
		PrivKey: privKey,
	}
}
