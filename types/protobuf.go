package types

import (
	"github.com/tendermint/abci/types"
)

// TM2PB is used for converting Tendermint types to protobuf types.
// UNSTABLE
var TM2PB = tm2pb{}

type tm2pb struct{}

func (tm2pb) Header(header *Header) types.Header {
	return types.Header{
		ChainID:        header.ChainID,
		Height:         header.Height,
		Time:           header.Time.Unix(),
		NumTxs:         int32(header.NumTxs), // XXX: overflow
		LastBlockID:    TM2PB.BlockID(header.LastBlockID),
		LastCommitHash: header.LastCommitHash,
		DataHash:       header.DataHash,
		AppHash:        header.AppHash,
	}
}

func (tm2pb) BlockID(blockID BlockID) types.BlockID {
	return types.BlockID{
		Hash:  blockID.Hash,
		Parts: TM2PB.PartSetHeader(blockID.PartsHeader),
	}
}

func (tm2pb) PartSetHeader(partSetHeader PartSetHeader) types.PartSetHeader {
	return types.PartSetHeader{
		Total: int32(partSetHeader.Total), // XXX: overflow
		Hash:  partSetHeader.Hash,
	}
}

func (tm2pb) Validator(val *Validator) types.Validator {
	return types.Validator{
		PubKey: val.PubKey.Bytes(),
		Power:  val.VotingPower,
	}
}

func (tm2pb) Validators(vals *ValidatorSet) []types.Validator {
	validators := make([]types.Validator, len(vals.Validators))
	for i, val := range vals.Validators {
		validators[i] = TM2PB.Validator(val)
	}
	return validators
}

func (tm2pb) ConsensusParams(params *ConsensusParams) *types.ConsensusParams {
	return &types.ConsensusParams{
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
