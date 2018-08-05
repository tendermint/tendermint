package types

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
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
		ChainID: header.ChainID,
		Height:  header.Height,

		Time:     header.Time,
		NumTxs:   int32(header.NumTxs), // XXX: overflow
		TotalTxs: header.TotalTxs,

		LastBlockHash:  header.LastBlockID.Hash,
		ValidatorsHash: header.ValidatorsHash,
		AppHash:        header.AppHash,

		// Proposer: TODO
	}
}

func (tm2pb) ValidatorWithoutPubKey(val *Validator) abci.Validator {
	return abci.Validator{
		Address: val.PubKey.Address(),
		Power:   val.VotingPower,
	}
}

// XXX: panics on unknown pubkey type
func (tm2pb) Validator(val *Validator) abci.Validator {
	return abci.Validator{
		Address: val.PubKey.Address(),
		PubKey:  TM2PB.PubKey(val.PubKey),
		Power:   val.VotingPower,
	}
}

// XXX: panics on nil or unknown pubkey type
// TODO: add cases when new pubkey types are added to crypto
func (tm2pb) PubKey(pubKey crypto.PubKey) abci.PubKey {
	switch pk := pubKey.(type) {
	case ed25519.PubKeyEd25519:
		return abci.PubKey{
			Type: ABCIPubKeyTypeEd25519,
			Data: pk[:],
		}
	case secp256k1.PubKeySecp256k1:
		return abci.PubKey{
			Type: ABCIPubKeyTypeSecp256k1,
			Data: pk[:],
		}
	default:
		panic(fmt.Sprintf("unknown pubkey type: %v %v", pubKey, reflect.TypeOf(pubKey)))
	}
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) Validators(vals *ValidatorSet) []abci.Validator {
	validators := make([]abci.Validator, vals.Size())
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
// XXX: panics on nil or unknown pubkey type
func (tm2pb) Evidence(ev Evidence, valSet *ValidatorSet, evTime time.Time) abci.Evidence {
	_, val := valSet.GetByAddress(ev.Address())
	if val == nil {
		// should already have checked this
		panic(val)
	}

	// set type
	var evType string
	switch ev.(type) {
	case *DuplicateVoteEvidence:
		evType = ABCIEvidenceTypeDuplicateVote
	case MockGoodEvidence:
		// XXX: not great to have test types in production paths ...
		evType = ABCIEvidenceTypeMockGood
	default:
		panic(fmt.Sprintf("Unknown evidence type: %v %v", ev, reflect.TypeOf(ev)))
	}

	return abci.Evidence{
		Type:             evType,
		Validator:        TM2PB.ValidatorWithoutPubKey(val),
		Height:           ev.Height(),
		Time:             evTime,
		TotalVotingPower: valSet.TotalVotingPower(),
	}
}

// XXX: panics on nil or unknown pubkey type
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
	// TODO: define these in crypto and use them
	sizeEd := 32
	sizeSecp := 33
	switch pubKey.Type {
	case ABCIPubKeyTypeEd25519:
		if len(pubKey.Data) != sizeEd {
			return nil, fmt.Errorf("Invalid size for PubKeyEd25519. Got %d, expected %d", len(pubKey.Data), sizeEd)
		}
		var pk ed25519.PubKeyEd25519
		copy(pk[:], pubKey.Data)
		return pk, nil
	case ABCIPubKeyTypeSecp256k1:
		if len(pubKey.Data) != sizeSecp {
			return nil, fmt.Errorf("Invalid size for PubKeyEd25519. Got %d, expected %d", len(pubKey.Data), sizeSecp)
		}
		var pk secp256k1.PubKeySecp256k1
		copy(pk[:], pubKey.Data)
		return pk, nil
	default:
		return nil, fmt.Errorf("Unknown pubkey type %v", pubKey.Type)
	}
}

func (pb2tm) Validators(vals []abci.Validator) ([]*Validator, error) {
	tmVals := make([]*Validator, len(vals))
	for i, v := range vals {
		pub, err := PB2TM.PubKey(v.PubKey)
		if err != nil {
			return nil, err
		}
		// If the app provided an address too, it must match.
		// This is just a sanity check.
		if len(v.Address) > 0 {
			if !bytes.Equal(pub.Address(), v.Address) {
				return nil, fmt.Errorf("Validator.Address (%X) does not match PubKey.Address (%X)",
					v.Address, pub.Address())
			}
		}
		tmVals[i] = &Validator{
			Address:     pub.Address(),
			PubKey:      pub,
			VotingPower: v.Power,
		}
	}
	return tmVals, nil
}

func (pb2tm) ConsensusParams(csp *abci.ConsensusParams) ConsensusParams {
	return ConsensusParams{
		BlockSize: BlockSize{
			MaxBytes: int(csp.BlockSize.MaxBytes), // XXX
			MaxTxs:   int(csp.BlockSize.MaxTxs),   // XXX
			MaxGas:   csp.BlockSize.MaxGas,
		},
		TxSize: TxSize{
			MaxBytes: int(csp.TxSize.MaxBytes), // XXX
			MaxGas:   csp.TxSize.MaxGas,
		},
		BlockGossip: BlockGossip{
			BlockPartSizeBytes: int(csp.BlockGossip.BlockPartSizeBytes), // XXX
		},
		// TODO: EvidenceParams: EvidenceParams{
		// MaxAge: int(csp.Evidence.MaxAge), // XXX
		// },
	}
}
