package types

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	crypto2 "github.com/tendermint/tendermint/proto/tendermint/crypto"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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

var ABCIPubKeyTypesToNames = map[string]string{
	ABCIPubKeyTypeEd25519:   ed25519.PubKeyName,
	ABCIPubKeyTypeSecp256k1: secp256k1.PubKeyName,
	ABCIPubKeyTypeBLS12381:  bls12381.PubKeyName,
}

//-------------------------------------------------------

// TM2PB is used for converting Tendermint ABCI to protobuf ABCI.
// UNSTABLE
var TM2PB = tm2pb{}

type tm2pb struct{}

func (tm2pb) Header(header *Header) tmproto.Header {
	return tmproto.Header{
		Version: header.Version,
		ChainID: header.ChainID,
		Height:  header.Height,
		Time:    header.Time,

		LastBlockId: header.LastBlockID.ToProto(),

		LastCommitHash: header.LastCommitHash,
		DataHash:       header.DataHash,

		ValidatorsHash:     header.ValidatorsHash,
		NextValidatorsHash: header.NextValidatorsHash,
		ConsensusHash:      header.ConsensusHash,
		AppHash:            header.AppHash,
		LastResultsHash:    header.LastResultsHash,

		EvidenceHash:      header.EvidenceHash,
		ProposerProTxHash: header.ProposerProTxHash,
	}
}

func (tm2pb) Validator(val *Validator) abci.Validator {
	return abci.Validator{
		Power:     val.VotingPower,
		ProTxHash: val.ProTxHash,
	}
}

func (tm2pb) BlockID(blockID BlockID) tmproto.BlockID {
	return tmproto.BlockID{
		Hash:          blockID.Hash,
		PartSetHeader: TM2PB.PartSetHeader(blockID.PartSetHeader),
	}
}

func (tm2pb) PartSetHeader(header PartSetHeader) tmproto.PartSetHeader {
	return tmproto.PartSetHeader{
		Total: header.Total,
		Hash:  header.Hash,
	}
}

// XXX: panics on unknown pubkey type
func (tm2pb) ValidatorUpdate(val *Validator) abci.ValidatorUpdate {
	pk, err := cryptoenc.PubKeyToProto(val.PubKey)
	if err != nil {
		panic(err)
	}
	return abci.ValidatorUpdate{
		PubKey:    pk,
		Power:     val.VotingPower,
		ProTxHash: val.ProTxHash,
	}
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) ValidatorUpdates(vals *ValidatorSet) abci.ValidatorSetUpdate {
	validators := make([]abci.ValidatorUpdate, vals.Size())
	for i, val := range vals.Validators {
		validators[i] = TM2PB.ValidatorUpdate(val)
	}
	abciThresholdPublicKey, err := cryptoenc.PubKeyToProto(vals.ThresholdPublicKey)
	if err != nil {
		panic(err)
	}
	return abci.ValidatorSetUpdate{ValidatorUpdates: validators, ThresholdPublicKey: abciThresholdPublicKey, QuorumHash: vals.QuorumHash}
}

func (tm2pb) ConsensusParams(params *tmproto.ConsensusParams) *abci.ConsensusParams {
	return &abci.ConsensusParams{
		Block: &abci.BlockParams{
			MaxBytes: params.Block.MaxBytes,
			MaxGas:   params.Block.MaxGas,
		},
		Evidence:  &params.Evidence,
		Validator: &params.Validator,
	}
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) NewValidatorUpdate(pubkey crypto.PubKey, power int64, proTxHash []byte) abci.ValidatorUpdate {
	pubkeyABCI, err := cryptoenc.PubKeyToProto(pubkey)
	if err != nil {
		panic(err)
	}
	return abci.ValidatorUpdate{
		PubKey:    pubkeyABCI,
		Power:     power,
		ProTxHash: proTxHash,
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
		pub, err := cryptoenc.PubKeyFromProto(v.PubKey)
		if err != nil {
			return nil, err
		}
		tmVals[i] = NewValidator(pub, v.Power, v.ProTxHash)
	}
	return tmVals, nil
}

func (pb2tm) ValidatorUpdatesFromValidatorSet(valSetUpdate *abci.ValidatorSetUpdate) ([]*Validator,
	crypto.PubKey, crypto.QuorumHash, error) {
	if valSetUpdate == nil {
		return nil, nil, nil, nil
	}
	tmVals := make([]*Validator, len(valSetUpdate.ValidatorUpdates))
	for i, v := range valSetUpdate.ValidatorUpdates {
		pub, err := cryptoenc.PubKeyFromProto(v.PubKey)
		if err != nil {
			return nil, nil, nil, err
		}
		tmVals[i] = NewValidator(pub, v.Power, v.ProTxHash)
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
