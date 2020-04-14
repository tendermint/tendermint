package types

import (
	"fmt"
	"reflect"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto/sr25519"
	tmproto "github.com/tendermint/tendermint/proto/types"
)

//-------------------------------------------------------
// Use strings to distinguish types in ABCI messages

const (
	ABCIEvidenceTypeDuplicateVote = "duplicate/vote"
	ABCIEvidenceTypeMock          = "mock/evidence"
)

const (
	ABCIPubKeyTypeEd25519   = "ed25519"
	ABCIPubKeyTypeSr25519   = "sr25519"
	ABCIPubKeyTypeSecp256k1 = "secp256k1"
)

// TODO: Make non-global by allowing for registration of more pubkey types

var ABCIPubKeyTypesToAminoNames = map[string]string{
	ABCIPubKeyTypeEd25519:   ed25519.PubKeyAminoName,
	ABCIPubKeyTypeSr25519:   sr25519.PubKeyAminoName,
	ABCIPubKeyTypeSecp256k1: secp256k1.PubKeyAminoName,
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

		LastBlockID: TM2PB.BlockID(header.LastBlockID),

		LastCommitHash: header.LastCommitHash,
		DataHash:       header.DataHash,

		ValidatorsHash:     header.ValidatorsHash,
		NextValidatorsHash: header.NextValidatorsHash,
		ConsensusHash:      header.ConsensusHash,
		AppHash:            header.AppHash,
		LastResultsHash:    header.LastResultsHash,

		EvidenceHash:    header.EvidenceHash,
		ProposerAddress: header.ProposerAddress,
	}
}

func (tm2pb) Validator(val *Validator) abci.Validator {
	return abci.Validator{
		Address: val.PubKey.Address(),
		Power:   val.VotingPower,
	}
}

func (tm2pb) BlockID(blockID BlockID) tmproto.BlockID {
	return tmproto.BlockID{
		Hash:        blockID.Hash,
		PartsHeader: TM2PB.PartSetHeader(blockID.PartsHeader),
	}
}

func (tm2pb) PartSetHeader(header PartSetHeader) tmproto.PartSetHeader {
	return tmproto.PartSetHeader{
		Total: uint32(header.Total),
		Hash:  header.Hash,
	}
}

// XXX: panics on unknown pubkey type
func (tm2pb) ValidatorUpdate(val *Validator) abci.ValidatorUpdate {
	return abci.ValidatorUpdate{
		PubKey: TM2PB.PubKey(val.PubKey),
		Power:  val.VotingPower,
	}
}

// XXX: panics on nil or unknown pubkey type
// TODO: add cases when new pubkey types are added to crypto
func (tm2pb) PubKey(pubKey crypto.PubKey) abci.PubKey {
	switch pk := pubKey.(type) {
	case ed25519.PubKey:
		return abci.PubKey{
			Type: ABCIPubKeyTypeEd25519,
			Data: pk[:],
		}
	case sr25519.PubKey:
		return abci.PubKey{
			Type: ABCIPubKeyTypeSr25519,
			Data: pk[:],
		}
	case secp256k1.PubKey:
		return abci.PubKey{
			Type: ABCIPubKeyTypeSecp256k1,
			Data: pk[:],
		}
	default:
		panic(fmt.Sprintf("unknown pubkey type: %v %v", pubKey, reflect.TypeOf(pubKey)))
	}
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) ValidatorUpdates(vals *ValidatorSet) []abci.ValidatorUpdate {
	validators := make([]abci.ValidatorUpdate, vals.Size())
	for i, val := range vals.Validators {
		validators[i] = TM2PB.ValidatorUpdate(val)
	}
	return validators
}

func (tm2pb) ConsensusParams(params *ConsensusParams) *abci.ConsensusParams {
	return &abci.ConsensusParams{
		Block: &abci.BlockParams{
			MaxBytes: params.Block.MaxBytes,
			MaxGas:   params.Block.MaxGas,
		},
		Evidence: &abci.EvidenceParams{
			MaxAgeNumBlocks: params.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  params.Evidence.MaxAgeDuration,
		},
		Validator: &abci.ValidatorParams{
			PubKeyTypes: params.Validator.PubKeyTypes,
		},
	}
}

// ABCI Evidence includes information from the past that's not included in the evidence itself
// so Evidence types stays compact.
// XXX: panics on nil or unknown pubkey type
func (tm2pb) Evidence(ev Evidence, valSet *ValidatorSet, evTime time.Time) abci.Evidence {
	_, val, ok := valSet.GetByAddress(ev.Address())
	if !ok {
		// should already have checked this
		panic(val)
	}

	// set type
	var evType string
	switch ev.(type) {
	case *DuplicateVoteEvidence:
		evType = ABCIEvidenceTypeDuplicateVote
	case MockEvidence:
		// XXX: not great to have test types in production paths ...
		evType = ABCIEvidenceTypeMock
	default:
		panic(fmt.Sprintf("Unknown evidence type: %v %v", ev, reflect.TypeOf(ev)))
	}

	return abci.Evidence{
		Type:             evType,
		Validator:        TM2PB.Validator(val),
		Height:           ev.Height(),
		Time:             evTime,
		TotalVotingPower: valSet.TotalVotingPower(),
	}
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) NewValidatorUpdate(pubkey crypto.PubKey, power int64) abci.ValidatorUpdate {
	pubkeyABCI := TM2PB.PubKey(pubkey)
	return abci.ValidatorUpdate{
		PubKey: pubkeyABCI,
		Power:  power,
	}
}

//----------------------------------------------------------------------------

// PB2TM is used for converting protobuf ABCI to Tendermint ABCI.
// UNSTABLE
var PB2TM = pb2tm{}

type pb2tm struct{}

func (pb2tm) PubKey(pubKey abci.PubKey) (crypto.PubKey, error) {
	switch pubKey.Type {
	case ABCIPubKeyTypeEd25519:
		if len(pubKey.Data) != ed25519.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeyEd25519. Got %d, expected %d",
				len(pubKey.Data), ed25519.PubKeySize)
		}
		var pk = make(ed25519.PubKey, ed25519.PubKeySize)
		copy(pk, pubKey.Data)
		return pk, nil
	case ABCIPubKeyTypeSr25519:
		if len(pubKey.Data) != sr25519.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeySr25519. Got %d, expected %d",
				len(pubKey.Data), sr25519.PubKeySize)
		}
		var pk = make(sr25519.PubKey, sr25519.PubKeySize)
		copy(pk, pubKey.Data)
		return pk, nil
	case ABCIPubKeyTypeSecp256k1:
		if len(pubKey.Data) != secp256k1.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeySecp256k1. Got %d, expected %d",
				len(pubKey.Data), secp256k1.PubKeySize)
		}
		var pk = make(secp256k1.PubKey, secp256k1.PubKeySize)
		copy(pk, pubKey.Data)
		return pk, nil
	default:
		return nil, fmt.Errorf("unknown pubkey type %v", pubKey.Type)
	}
}

func (pb2tm) ValidatorUpdates(vals []abci.ValidatorUpdate) ([]*Validator, error) {
	tmVals := make([]*Validator, len(vals))
	for i, v := range vals {
		pub, err := PB2TM.PubKey(v.PubKey)
		if err != nil {
			return nil, err
		}
		tmVals[i] = NewValidator(pub, v.Power)
	}
	return tmVals, nil
}
