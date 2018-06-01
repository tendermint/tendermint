package types

import (
	"fmt"
	"reflect"

	"github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
)

// TM2PB is used for converting Tendermint types to protobuf types.
// UNSTABLE
var TM2PB = tm2pb{}

type tm2pb struct{}

func (tm2pb) Header(header *Header) types.Header {
	return types.Header{
		ChainID:       header.ChainID,
		Height:        header.Height,
		Time:          header.Time.Unix(),
		NumTxs:        int32(header.NumTxs), // XXX: overflow
		LastBlockHash: header.LastBlockID.Hash,
		AppHash:       header.AppHash,
	}
}

func (tm2pb) Validator(val *Validator) types.Validator {
	return types.Validator{
		PubKey: TM2PB.PubKey(val.PubKey),
		Power:  val.VotingPower,
	}
}

func (tm2pb) PubKey(pubKey crypto.PubKey) types.PubKey {
	switch pk := pubKey.(type) {
	case crypto.PubKeyEd25519:
		return types.PubKey{
			Type: "ed25519",
			Data: pk[:],
		}
	case crypto.PubKeySecp256k1:
		return types.PubKey{
			Type: "secp256k1",
			Data: pk[:],
		}
	default:
		panic(fmt.Sprintf("unknown pubkey type: %v %v", pubKey, reflect.TypeOf(pubKey)))
	}
}

func (tm2pb) Validators(vals *ValidatorSet) []types.Validator {
	validators := make([]types.Validator, len(vals.Validators))
	for i, val := range vals.Validators {
		validators[i] = TM2PB.Validator(val)
	}
	return validators
}

func (tm2pb) ConsensusParams(params *ConsensusParams) types.ConsensusParams {
	return types.ConsensusParams{
		BlockSize: &types.BlockSize{

			MaxBytes: int32(params.BlockSize.MaxBytes),
			MaxTxs:   int32(params.BlockSize.MaxTxs),
			MaxGas:   params.BlockSize.MaxGas,
		},
		TxSize: &types.TxSize{
			MaxBytes: int32(params.TxSize.MaxBytes),
			MaxGas:   params.TxSize.MaxGas,
		},
		BlockGossip: &types.BlockGossip{
			BlockPartSizeBytes: int32(params.BlockGossip.BlockPartSizeBytes),
		},
	}
}

//----------------------------------------------------------------------------

// PB2TM is used for converting protobuf types to Tendermint types.
// UNSTABLE
var PB2TM = pb2tm{}

type pb2tm struct{}

// TODO: validate key lengths ...
func (pb2tm) PubKey(pubKey types.PubKey) (crypto.PubKey, error) {
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
