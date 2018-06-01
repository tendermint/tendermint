package types

import (
	"fmt"
	"reflect"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
)

// TM2PB is used for converting Tendermint abci to protobuf abci.
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

func (tm2pb) Evidence(ev_ Evidence) abci.Evidence {
	switch ev := ev_.(type) {
	case *DuplicateVoteEvidence:
		return abci.Evidence{
			Type: "duplicate/vote",
			Validator: abci.Validator{
				Address: ev.Address(),
				// TODO
			},
			Height: ev.Height(),
			//	Time:             ev.Time(),
			//	TotalVotingPower: 10,
		}
	default:
		panic(fmt.Sprintf("Unknown evidence type: %v %v", ev_, reflect.TypeOf(ev_)))
	}

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

// PB2TM is used for converting protobuf abci to Tendermint abci.
// UNSTABLE
var PB2TM = pb2tm{}

type pb2tm struct{}

// TODO: validate key lengths ...
func (pb2tm) PubKey(pubKey abci.PubKey) (crypto.PubKey, error) {
	switch pubKey.Type {
	case "ed25519":
		var pk crypto.PubKeyEd25519
		copy(pk[:], pubKey.Data)
		return pk, nil
	case "secp256k1":
		var pk crypto.PubKeySecp256k1
		copy(pk[:], pubKey.Data)
		return pk, nil
	default:
		return nil, fmt.Errorf("Unknown pubkey type %v", pubKey.Type)
	}
}
