package types

import (
	"errors"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	errUnexpectedVoteType = errors.New("unexpected vote extension - vote extensions are only allowed in precommits")
)

// QuorumSigns holds data which is necessary for signing and verification block, state, and each vote-extension in a list
type QuorumSigns struct {
	Block      SignItem
	State      SignItem
	Extensions map[VoteExtensionType][]SignItem
}

// SignItem represents quorum sing data, like a request id, message bytes, sha256 hash of message and signID
type SignItem struct {
	ReqID []byte
	ID    []byte
	Hash  []byte
	Raw   []byte
}

// MakeQuorumSignsWithVoteSet creates and returns QuorumSigns struct built with a vote-set and an added vote
func MakeQuorumSignsWithVoteSet(voteSet *VoteSet, vote *types.Vote) (QuorumSigns, error) {
	return MakeQuorumSigns(
		voteSet.chainID,
		voteSet.valSet.QuorumType,
		voteSet.valSet.QuorumHash,
		vote,
		voteSet.stateID,
	)
}

// MakeQuorumSigns builds signing data for block, state and vote-extensions
// each a sign-id item consist of request-id, raw data, hash of raw and id
func MakeQuorumSigns(
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	protoVote *types.Vote,
	stateID StateID,
) (QuorumSigns, error) {
	quorumSign := QuorumSigns{
		Block: MakeBlockSignItem(chainID, protoVote, quorumType, quorumHash),
		State: MakeStateSignItem(chainID, stateID, quorumType, quorumHash),
	}
	var err error
	quorumSign.Extensions, err = MakeVoteExtensionSignItems(chainID, protoVote, quorumType, quorumHash)
	if err != nil {
		return QuorumSigns{}, err
	}
	return quorumSign, nil
}

// MakeBlockSignItem creates SignItem struct for a block
func MakeBlockSignItem(chainID string, vote *types.Vote, quorumType btcjson.LLMQType, quorumHash []byte) SignItem {
	reqID := voteHeightRoundRequestID("dpbvote", vote.Height, vote.Round)
	raw := VoteBlockSignBytes(chainID, vote)
	return newSignItem(quorumType, quorumHash, reqID, raw)
}

// MakeStateSignItem creates SignItem struct for a state
func MakeStateSignItem(chainID string, stateID StateID, quorumType btcjson.LLMQType, quorumHash []byte) SignItem {
	reqID := stateID.SignRequestID()
	raw := stateID.SignBytes(chainID)
	return newSignItem(quorumType, quorumHash, reqID, raw)
}

// MakeVoteExtensionSignItems  creates a list SignItem structs for a vote extensions
func MakeVoteExtensionSignItems(chainID string, protoVote *types.Vote, quorumType btcjson.LLMQType, quorumHash []byte) (map[VoteExtensionType][]SignItem, error) {
	// We only sign vote extensions for precommits
	if protoVote.Type != types.PrecommitType {
		if !protoVote.VoteExtensions.IsEmpty() {
			return nil, errUnexpectedVoteType
		}
		return nil, nil
	}
	items := make(map[VoteExtensionType][]SignItem)
	reqID := VoteExtensionRequestID(protoVote)
	protoMap := ProtoVoteExtensionsToMap(protoVote.VoteExtensions)
	for t, exts := range protoMap {
		if items[t] == nil && len(exts) > 0 {
			items[t] = make([]SignItem, len(exts))
		}
		for i, ext := range exts {
			raw := VoteExtensionSignBytes(chainID, protoVote.Height, protoVote.Round, ext)
			items[t][i] = newSignItem(quorumType, quorumHash, reqID, raw)
		}
	}
	return items, nil
}

func newSignItem(quorumType btcjson.LLMQType, quorumHash, reqID, raw []byte) SignItem {
	return SignItem{
		ReqID: reqID,
		ID:    makeSignID(raw, reqID, quorumType, quorumHash),
		Hash:  crypto.Checksum(raw),
		Raw:   raw,
	}
}
