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
	Block    SignItem
	State    SignItem
	VoteExts []SignItem
}

// SignItem represents quorum sign data, like a request id, message bytes, sha256 hash of message and signID
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
	quorumSign.VoteExts, err = MakeVoteExtensionSignItems(chainID, protoVote, quorumType, quorumHash)
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
func MakeVoteExtensionSignItems(chainID string, protoVote *types.Vote, quorumType btcjson.LLMQType, quorumHash []byte) ([]SignItem, error) {
	// We only sign vote extensions for precommits
	if protoVote.Type != types.PrecommitType {
		if len(protoVote.VoteExtensions) > 0 {
			return nil, errUnexpectedVoteType
		}
		return nil, nil
	}
	items := make([]SignItem, len(protoVote.VoteExtensions))
	reqID := VoteExtensionRequestID(protoVote)
	for i, ext := range protoVote.VoteExtensions {
		raw := VoteExtensionSignBytes(chainID, protoVote.Height, protoVote.Round, ext)
		items[i] = newSignItem(quorumType, quorumHash, reqID, raw)
	}
	return items, nil
}

// GetRecoverableSingItems returns a list of SignItems only for recoverable vote extensions
func GetRecoverableSingItems(exts []VoteExtension, signItems []SignItem) []SignItem {
	var res []SignItem
	for i, ext := range exts {
		if ext.IsRecoverable() {
			res = append(res, signItems[i])
		}
	}
	return res
}

func newSignItem(quorumType btcjson.LLMQType, quorumHash, reqID, raw []byte) SignItem {
	return SignItem{
		ReqID: reqID,
		ID:    makeSignID(raw, reqID, quorumType, quorumHash),
		Hash:  crypto.Checksum(raw),
		Raw:   raw,
	}
}
