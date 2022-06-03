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

// SignIDs holds all possible signID, like blockID, stateID and signID for each vote-extensions
type SignIDs struct {
	BlockID    SignIDItem
	StateID    SignIDItem
	VoteExtIDs []SignIDItem
}

// SignIDItem represents quorum sing data, like a request id, message bytes, sha256 hash of message and signID
type SignIDItem struct {
	ReqID []byte
	ID    []byte
	Hash  []byte
	Raw   []byte
}

// MakeSignIDsWithVoteSet creates and returns SignIDs struct built with a vote-set and an added vote
func MakeSignIDsWithVoteSet(voteSet *VoteSet, vote *types.Vote) (SignIDs, error) {
	return MakeSignIDs(
		voteSet.chainID,
		voteSet.valSet.QuorumType,
		voteSet.valSet.QuorumHash,
		vote,
		voteSet.stateID,
	)
}

// MakeSignIDs builds signing data for block, state and vote-extensions
// each a sign-id item consist of request-id, raw data, hash of raw and id
func MakeSignIDs(
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	protoVote *types.Vote,
	stateID StateID,
) (SignIDs, error) {
	signIDs := SignIDs{
		BlockID: MakeBlockSignID(chainID, protoVote, quorumType, quorumHash),
		StateID: MakeStateSignID(chainID, stateID, quorumType, quorumHash),
	}
	var err error
	signIDs.VoteExtIDs, err = MakeVoteExtensionSignIDs(chainID, protoVote, quorumType, quorumHash)
	if err != nil {
		return SignIDs{}, err
	}
	return signIDs, nil
}

// MakeBlockSignID creates SignIDItem struct for a block
func MakeBlockSignID(chainID string, vote *types.Vote, quorumType btcjson.LLMQType, quorumHash []byte) SignIDItem {
	reqID := voteHeightRoundRequestID("dpbvote", vote.Height, vote.Round)
	raw := VoteBlockSignBytes(chainID, vote)
	return newSignIDItem(quorumType, quorumHash, reqID, raw)
}

// MakeStateSignID creates SignIDItem struct for a state
func MakeStateSignID(chainID string, stateID StateID, quorumType btcjson.LLMQType, quorumHash []byte) SignIDItem {
	reqID := stateID.SignRequestID()
	raw := stateID.SignBytes(chainID)
	return newSignIDItem(quorumType, quorumHash, reqID, raw)
}

// MakeVoteExtensionSignIDs  creates a list SignIDItem structs for a vote extensions
func MakeVoteExtensionSignIDs(chainID string, protoVote *types.Vote, quorumType btcjson.LLMQType, quorumHash []byte) ([]SignIDItem, error) {
	// We only sign vote extensions for precommits
	if protoVote.Type != types.PrecommitType {
		if len(protoVote.VoteExtensions) > 0 {
			return nil, errUnexpectedVoteType
		}
		return nil, nil
	}
	items := make([]SignIDItem, len(protoVote.VoteExtensions))
	reqID := VoteExtensionRequestID(protoVote)
	for i, ext := range protoVote.VoteExtensions {
		raw := VoteExtensionSignBytes(chainID, protoVote.Height, protoVote.Round, ext)
		items[i] = newSignIDItem(quorumType, quorumHash, reqID, raw)
	}
	return items, nil
}

func newSignIDItem(quorumType btcjson.LLMQType, quorumHash, reqID, raw []byte) SignIDItem {
	return SignIDItem{
		ReqID: reqID,
		ID:    makeSignID(raw, reqID, quorumType, quorumHash),
		Hash:  crypto.Checksum(raw),
		Raw:   raw,
	}
}
