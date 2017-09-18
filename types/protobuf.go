// Copyright 2016 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"github.com/tendermint/abci/types"
)

// Convert tendermint types to protobuf types
var TM2PB = tm2pb{}

type tm2pb struct{}

func (tm2pb) Header(header *Header) *types.Header {
	return &types.Header{
		ChainId:        header.ChainID,
		Height:         uint64(header.Height),
		Time:           uint64(header.Time.Unix()),
		NumTxs:         uint64(header.NumTxs),
		LastBlockId:    TM2PB.BlockID(header.LastBlockID),
		LastCommitHash: header.LastCommitHash,
		DataHash:       header.DataHash,
		AppHash:        header.AppHash,
	}
}

func (tm2pb) BlockID(blockID BlockID) *types.BlockID {
	return &types.BlockID{
		Hash:  blockID.Hash,
		Parts: TM2PB.PartSetHeader(blockID.PartsHeader),
	}
}

func (tm2pb) PartSetHeader(partSetHeader PartSetHeader) *types.PartSetHeader {
	return &types.PartSetHeader{
		Total: uint64(partSetHeader.Total),
		Hash:  partSetHeader.Hash,
	}
}

func (tm2pb) Validator(val *Validator) *types.Validator {
	return &types.Validator{
		PubKey: val.PubKey.Bytes(),
		Power:  uint64(val.VotingPower),
	}
}

func (tm2pb) Validators(vals *ValidatorSet) []*types.Validator {
	validators := make([]*types.Validator, len(vals.Validators))
	for i, val := range vals.Validators {
		validators[i] = TM2PB.Validator(val)
	}
	return validators
}
