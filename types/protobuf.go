package types

import (
	"fmt"
	"reflect"
	"time"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
)

//-------------------------------------------------------
// Use strings to distinguish types in ABCI messages

const (
	ABCIEvidenceTypeDuplicateVote = "duplicate/vote"
	ABCIEvidenceTypeMockGood      = "mock/good"
)

const (
	ABCIPubKeyTypeEd25519   = "ed25519"
	ABCIPubKeyTypeSecp256k1 = "secp256k1"
)

//-------------------------------------------------------

// TM2PB is used for converting Tendermint ABCI to protobuf ABCI.
// UNSTABLE
var TM2PB = tm2pb{}

type tm2pb struct{}

func (tm2pb) Header(header *Header) abci.Header {
	return abci.Header{
		ChainID:       header.ChainID,
		Height:        header.Height,
		Time:          header.Time.Unix(),
		NumTxs:        int32(header.NumTxs), // XXX: overflow
		LastBlockHash: header.LastBlockID.Hash,
		AppHash:       header.AppHash,
	}
}

func (tm2pb) Validator(val *Validator) abci.Validator {
	return abci.Validator{
		PubKey: TM2PB.PubKey(val.PubKey),
		Power:  val.VotingPower,
	}
}

func (tm2pb) PubKey(pubKey crypto.PubKey) abci.PubKey {
	switch pk := pubKey.(type) {
	case crypto.PubKeyEd25519:
		return abci.PubKey{
			Type: "ed25519",
			Data: pk[:],
		}
	case crypto.PubKeySecp256k1:
		return abci.PubKey{
			Type: "secp256k1",
			Data: pk[:],
		}
	default:
		panic(fmt.Sprintf("unknown pubkey type: %v %v", pubKey, reflect.TypeOf(pubKey)))
	}
}

func (tm2pb) Validators(vals *ValidatorSet) []abci.Validator {
	validators := make([]abci.Validator, len(vals.Validators))
	for i, val := range vals.Validators {
		validators[i] = TM2PB.Validator(val)
	}
	return validators
}

func (tm2pb) ConsensusParams(params *ConsensusParams) *abci.ConsensusParams {
	return &abci.ConsensusParams{
		BlockSize: &abci.BlockSize{

			MaxBytes: int32(params.BlockSize.MaxBytes),
			MaxTxs:   int32(params.BlockSize.MaxTxs),
			MaxGas:   params.BlockSize.MaxGas,
		},
		TxSize: &abci.TxSize{
			MaxBytes: int32(params.TxSize.MaxBytes),
			MaxGas:   params.TxSize.MaxGas,
		},
		BlockGossip: &abci.BlockGossip{
			BlockPartSizeBytes: int32(params.BlockGossip.BlockPartSizeBytes),
		},
	}
}

// ABCI Evidence includes information from the past that's not included in the evidence itself
// so Evidence types stays compact.
func (tm2pb) Evidence(ev Evidence, valSet *ValidatorSet, evTime time.Time) abci.Evidence {
	_, val := valSet.GetByAddress(ev.Address())
	if val == nil {
		// should already have checked this
		panic(val)
	}

	abciEvidence := abci.Evidence{
		Validator: abci.Validator{
			Address: ev.Address(),
			PubKey:  TM2PB.PubKey(val.PubKey),
			Power:   val.VotingPower,
		},
		Height:           ev.Height(),
		Time:             evTime.Unix(),
		TotalVotingPower: valSet.TotalVotingPower(),
	}

	// set type
	switch ev.(type) {
	case *DuplicateVoteEvidence:
		abciEvidence.Type = ABCIEvidenceTypeDuplicateVote
	case *MockGoodEvidence, MockGoodEvidence:
		abciEvidence.Type = ABCIEvidenceTypeMockGood
	default:
		panic(fmt.Sprintf("Unknown evidence type: %v %v", ev, reflect.TypeOf(ev)))
	}

	return abciEvidence
}

func (tm2pb) ValidatorFromPubKeyAndPower(pubkey crypto.PubKey, power int64) abci.Validator {
	pubkeyABCI := TM2PB.PubKey(pubkey)
	return abci.Validator{
		Address: pubkey.Address(),
		PubKey:  pubkeyABCI,
		Power:   power,
	}
}

//----------------------------------------------------------------------------

// PB2TM is used for converting protobuf ABCI to Tendermint ABCI.
// UNSTABLE
var PB2TM = pb2tm{}

type pb2tm struct{}

func (pb2tm) PubKey(pubKey abci.PubKey) (crypto.PubKey, error) {
	// TODO: define these in go-crypto and use them
	sizeEd := 32
	sizeSecp := 33
	switch pubKey.Type {
	case ABCIPubKeyTypeEd25519:
		if len(pubKey.Data) != sizeEd {
			return nil, fmt.Errorf("Invalid size for PubKeyEd25519. Got %d, expected %d", len(pubKey.Data), sizeEd)
		}
		var pk crypto.PubKeyEd25519
		copy(pk[:], pubKey.Data)
		return pk, nil
	case ABCIPubKeyTypeSecp256k1:
		if len(pubKey.Data) != sizeSecp {
			return nil, fmt.Errorf("Invalid size for PubKeyEd25519. Got %d, expected %d", len(pubKey.Data), sizeSecp)
		}
		var pk crypto.PubKeySecp256k1
		copy(pk[:], pubKey.Data)
		return pk, nil
	default:
		return nil, fmt.Errorf("Unknown pubkey type %v", pubKey.Type)
	}
}
