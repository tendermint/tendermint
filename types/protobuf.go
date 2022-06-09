package types

import (
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	crypto2 "github.com/tendermint/tendermint/proto/tendermint/crypto"
)

//-------------------------------------------------------
// Use strings to distinguish types in ABCI messages

const (
	ABCIPubKeyTypeEd25519            = ed25519.KeyType
	ABCIPubKeyTypeSecp256k1          = secp256k1.KeyType
	ABCIPubKeyTypeBLS12381           = bls12381.KeyType
	ABCIEvidenceTypeDuplicateVote    = "duplicate/vote"
	ABCIEvidenceTypePhantom          = "phantom"
	ABCIEvidenceTypeLunatic          = "lunatic"
	ABCIEvidenceTypePotentialAmnesia = "potential_amnesia"
	ABCIEvidenceTypeMock             = "mock/evidence"
)

// TODO: Make non-global by allowing for registration of more pubkey types

//-------------------------------------------------------

// TM2PB is used for converting Tendermint ABCI to protobuf ABCI.
// UNSTABLE
var TM2PB = tm2pb{}

type tm2pb struct{}

func (tm2pb) Validator(val *Validator) abci.Validator {
	return abci.Validator{
		Power:     val.VotingPower,
		ProTxHash: val.ProTxHash,
	}
}

// ValidatorUpdate panics on unknown pubkey type
func (tm2pb) ValidatorUpdate(val *Validator) abci.ValidatorUpdate {
	valUpdate := abci.ValidatorUpdate{
		Power:       val.VotingPower,
		ProTxHash:   val.ProTxHash,
		NodeAddress: val.NodeAddress.String(),
	}
	if val.PubKey != nil {
		pk := cryptoenc.MustPubKeyToProto(val.PubKey)
		valUpdate.PubKey = &pk
	}
	return valUpdate
}

// ValidatorUpdates panics on unknown pubkey type
func (tm2pb) ValidatorUpdates(vals *ValidatorSet) abci.ValidatorSetUpdate {
	validators := make([]abci.ValidatorUpdate, vals.Size())
	for i, val := range vals.Validators {
		validators[i] = TM2PB.ValidatorUpdate(val)
	}
	abciThresholdPublicKey := cryptoenc.MustPubKeyToProto(vals.ThresholdPublicKey)
	return abci.ValidatorSetUpdate{
		ValidatorUpdates:   validators,
		ThresholdPublicKey: abciThresholdPublicKey,
		QuorumHash:         vals.QuorumHash,
	}
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) NewValidatorUpdate(
	pubkey crypto.PubKey,
	power int64,
	proTxHash []byte,
	address string,
) abci.ValidatorUpdate {
	var pubkeyABCI *crypto2.PublicKey
	if pubkey != nil {
		pubkeyProto := cryptoenc.MustPubKeyToProto(pubkey)
		pubkeyABCI = &pubkeyProto
	} else {
		pubkeyABCI = nil
	}

	return abci.ValidatorUpdate{
		PubKey:      pubkeyABCI,
		Power:       power,
		ProTxHash:   proTxHash,
		NodeAddress: address,
	}
}

//----------------------------------------------------------------------------

// PB2TM is used for converting protobuf ABCI to Tendermint ABCI.
// UNSTABLE
var PB2TM = pb2tm{}

type pb2tm struct{}

func (pb2tm) ValidatorUpdates(vals []abci.ValidatorUpdate) ([]*Validator, error) {
	tmVals := make([]*Validator, len(vals))
	for i, v := range vals {
		pub := crypto.PubKey(nil)
		var err error
		if v.PubKey != nil {
			pub, err = cryptoenc.PubKeyFromProto(*v.PubKey)
			if err != nil {
				return nil, err
			}
		}
		tmVals[i] = NewValidator(pub, v.Power, v.ProTxHash, v.NodeAddress)
	}
	return tmVals, nil
}

func (pb2tm) ValidatorUpdatesFromValidatorSet(valSetUpdate *abci.ValidatorSetUpdate) (
	[]*Validator,
	crypto.PubKey,
	crypto.QuorumHash,
	error,
) {
	if valSetUpdate == nil {
		return nil, nil, nil, nil
	}
	tmVals := make([]*Validator, len(valSetUpdate.ValidatorUpdates))
	for i, v := range valSetUpdate.ValidatorUpdates {
		pub := crypto.PubKey(nil)
		var err error
		if v.PubKey != nil {
			pub, err = cryptoenc.PubKeyFromProto(*v.PubKey)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		tmVals[i] = NewValidator(pub, v.Power, v.ProTxHash, v.NodeAddress)
		err = tmVals[i].ValidateBasic()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("validator updates from validator set error when validating validator: %s", err)
		}
	}
	if valSetUpdate.ThresholdPublicKey.Sum == nil {
		return nil, nil, nil, nil
	}
	pub, err := cryptoenc.PubKeyFromProto(valSetUpdate.ThresholdPublicKey)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(valSetUpdate.QuorumHash) != crypto.DefaultHashSize {
		return nil, nil, nil, fmt.Errorf("validator set update must have a quorum"+
			" hash of 32 bytes (size: %d bytes)", len(valSetUpdate.QuorumHash))
	}
	return tmVals, pub, valSetUpdate.QuorumHash, nil
}

func (pb2tm) ValidatorSetFromProtoUpdate(
	quorumType btcjson.LLMQType,
	valSetUpdate *abci.ValidatorSetUpdate,
) (*ValidatorSet, error) {
	hasPublicKeys := true
	for _, v := range valSetUpdate.ValidatorUpdates {
		if v.PubKey == nil {
			hasPublicKeys = false
			break
		}
	}
	tmVals, pub, quorumHash, err := PB2TM.ValidatorUpdatesFromValidatorSet(valSetUpdate)
	if err != nil {
		return nil, err
	}
	return NewValidatorSet(tmVals, pub, quorumType, quorumHash, hasPublicKeys), nil
}

func (pb2tm) ThresholdPublicKeyUpdate(thresholdPublicKey crypto2.PublicKey) (crypto.PubKey, error) {
	if thresholdPublicKey.Sum == nil {
		return nil, nil
	}
	pub, err := cryptoenc.PubKeyFromProto(thresholdPublicKey)
	if err != nil {
		return nil, err
	}
	return pub, nil
}
