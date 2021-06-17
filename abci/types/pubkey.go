package types

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	crypto2 "github.com/tendermint/tendermint/proto/tendermint/crypto"
)

func UpdateValidator(proTxHash []byte, pk []byte, power int64) ValidatorUpdate {
	pke := bls12381.PubKey(pk)
	pkp, err := cryptoenc.PubKeyToProto(pke)
	if err != nil {
		panic(err)
	}

	return ValidatorUpdate{
		PubKey:    pkp,
		Power:     power,
		ProTxHash: proTxHash,
	}
}

func UpdateValidatorSet(validatorUpdates []ValidatorUpdate, thresholdPublicKey crypto2.PublicKey) ValidatorSetUpdate {
	return ValidatorSetUpdate{
		ValidatorUpdates:   validatorUpdates,
		ThresholdPublicKey: thresholdPublicKey,
	}
}

func UpdateThresholdPublicKey(pk []byte) ThresholdPublicKeyUpdate {
	pke := bls12381.PubKey(pk)
	pkp, err := cryptoenc.PubKeyToProto(pke)
	if err != nil {
		panic(err)
	}

	return ThresholdPublicKeyUpdate{
		ThresholdPublicKey: pkp,
	}
}

func UpdateQuorumHash(quorumHash crypto.QuorumHash) QuorumHashUpdate {
	return QuorumHashUpdate{
		QuorumHash: quorumHash,
	}
}
