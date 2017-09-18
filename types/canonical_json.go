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
	"github.com/tendermint/go-wire/data"
)

// canonical json is go-wire's json for structs with fields in alphabetical order

type CanonicalJSONBlockID struct {
	Hash        data.Bytes                 `json:"hash,omitempty"`
	PartsHeader CanonicalJSONPartSetHeader `json:"parts,omitempty"`
}

type CanonicalJSONPartSetHeader struct {
	Hash  data.Bytes `json:"hash"`
	Total int        `json:"total"`
}

type CanonicalJSONProposal struct {
	BlockPartsHeader CanonicalJSONPartSetHeader `json:"block_parts_header"`
	Height           int                        `json:"height"`
	POLBlockID       CanonicalJSONBlockID       `json:"pol_block_id"`
	POLRound         int                        `json:"pol_round"`
	Round            int                        `json:"round"`
}

type CanonicalJSONVote struct {
	BlockID CanonicalJSONBlockID `json:"block_id"`
	Height  int                  `json:"height"`
	Round   int                  `json:"round"`
	Type    byte                 `json:"type"`
}

type CanonicalJSONHeartbeat struct {
	Height           int        `json:"height"`
	Round            int        `json:"round"`
	Sequence         int        `json:"sequence"`
	ValidatorAddress data.Bytes `json:"validator_address"`
	ValidatorIndex   int        `json:"validator_index"`
}

//------------------------------------
// Messages including a "chain id" can only be applied to one chain, hence "Once"

type CanonicalJSONOnceProposal struct {
	ChainID  string                `json:"chain_id"`
	Proposal CanonicalJSONProposal `json:"proposal"`
}

type CanonicalJSONOnceVote struct {
	ChainID string            `json:"chain_id"`
	Vote    CanonicalJSONVote `json:"vote"`
}

type CanonicalJSONOnceHeartbeat struct {
	ChainID   string                 `json:"chain_id"`
	Heartbeat CanonicalJSONHeartbeat `json:"heartbeat"`
}

//-----------------------------------
// Canonicalize the structs

func CanonicalBlockID(blockID BlockID) CanonicalJSONBlockID {
	return CanonicalJSONBlockID{
		Hash:        blockID.Hash,
		PartsHeader: CanonicalPartSetHeader(blockID.PartsHeader),
	}
}

func CanonicalPartSetHeader(psh PartSetHeader) CanonicalJSONPartSetHeader {
	return CanonicalJSONPartSetHeader{
		psh.Hash,
		psh.Total,
	}
}

func CanonicalProposal(proposal *Proposal) CanonicalJSONProposal {
	return CanonicalJSONProposal{
		BlockPartsHeader: CanonicalPartSetHeader(proposal.BlockPartsHeader),
		Height:           proposal.Height,
		POLBlockID:       CanonicalBlockID(proposal.POLBlockID),
		POLRound:         proposal.POLRound,
		Round:            proposal.Round,
	}
}

func CanonicalVote(vote *Vote) CanonicalJSONVote {
	return CanonicalJSONVote{
		CanonicalBlockID(vote.BlockID),
		vote.Height,
		vote.Round,
		vote.Type,
	}
}

func CanonicalHeartbeat(heartbeat *Heartbeat) CanonicalJSONHeartbeat {
	return CanonicalJSONHeartbeat{
		heartbeat.Height,
		heartbeat.Round,
		heartbeat.Sequence,
		heartbeat.ValidatorAddress,
		heartbeat.ValidatorIndex,
	}
}
