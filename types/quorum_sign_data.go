package types

import (
	"errors"
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	errUnexpectedVoteType = errors.New("unexpected vote extension - vote extensions are only allowed in precommits")
)

// QuorumSignData holds data which is necessary for signing and verification block, state, and each vote-extension in a list
type QuorumSignData struct {
	Block      SignItem
	State      SignItem
	Extensions map[types.VoteExtensionType][]SignItem
}

// Verify verifies a quorum signatures: block, state and vote-extensions
func (q QuorumSignData) Verify(pubKey crypto.PubKey, signs QuorumSigns) error {
	return NewQuorumSingsVerifier(q).Verify(pubKey, signs)
}

// SignItem represents quorum sign data, like a request id, message bytes, sha256 hash of message and signID
type SignItem struct {
	ReqID []byte
	ID    []byte
	Raw   []byte
	Hash  []byte
}

// Validate validates prepared data for signing
func (i *SignItem) Validate() error {
	if len(i.ReqID) != crypto.DefaultHashSize {
		return fmt.Errorf("invalid request ID size: %X", i.ReqID)
	}
	return nil
}

// MakeQuorumSignsWithVoteSet creates and returns QuorumSignData struct built with a vote-set and an added vote
func MakeQuorumSignsWithVoteSet(voteSet *VoteSet, vote *types.Vote) (QuorumSignData, error) {
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
) (QuorumSignData, error) {
	quorumSign := QuorumSignData{
		Block: MakeBlockSignItem(chainID, protoVote, quorumType, quorumHash),
		State: MakeStateSignItem(chainID, stateID, quorumType, quorumHash),
	}
	var err error
	quorumSign.Extensions, err = MakeVoteExtensionSignItems(chainID, protoVote, quorumType, quorumHash)
	if err != nil {
		return QuorumSignData{}, err
	}
	return quorumSign, nil
}

// MakeBlockSignItem creates SignItem struct for a block
func MakeBlockSignItem(chainID string, vote *types.Vote, quorumType btcjson.LLMQType, quorumHash []byte) SignItem {
	reqID := voteHeightRoundRequestID("dpbvote", vote.Height, vote.Round)
	raw := VoteBlockSignBytes(chainID, vote)
	return NewSignItem(quorumType, quorumHash, reqID, raw)
}

// MakeStateSignItem creates SignItem struct for a state
func MakeStateSignItem(chainID string, stateID StateID, quorumType btcjson.LLMQType, quorumHash []byte) SignItem {
	reqID := stateID.SignRequestID()
	raw := stateID.SignBytes(chainID)
	return NewSignItem(quorumType, quorumHash, reqID, raw)
}

// MakeVoteExtensionSignItems  creates a list SignItem structs for a vote extensions
func MakeVoteExtensionSignItems(
	chainID string,
	protoVote *types.Vote,
	quorumType btcjson.LLMQType,
	quorumHash []byte,
) (map[types.VoteExtensionType][]SignItem, error) {
	// We only sign vote extensions for precommits
	if protoVote.Type != types.PrecommitType {
		if len(protoVote.VoteExtensions) > 0 {
			return nil, errUnexpectedVoteType
		}
		return nil, nil
	}
	items := make(map[types.VoteExtensionType][]SignItem)
	reqID := VoteExtensionRequestID(protoVote)
	protoExtensionsMap := protoVote.VoteExtensionsToMap()
	for t, exts := range protoExtensionsMap {
		if items[t] == nil && len(exts) > 0 {
			items[t] = make([]SignItem, len(exts))
		}
		for i, ext := range exts {
			raw := VoteExtensionSignBytes(chainID, protoVote.Height, protoVote.Round, ext)
			items[t][i] = NewSignItem(quorumType, quorumHash, reqID, raw)
		}
	}
	return items, nil
}

// NewSignItem creates a new instance of SignItem with calculating a hash for a raw and creating signID
func NewSignItem(quorumType btcjson.LLMQType, quorumHash, reqID, raw []byte) SignItem {
	msgHash := crypto.Checksum(raw)
	return SignItem{
		ReqID: reqID,
		ID:    MakeSignID(msgHash, reqID, quorumType, quorumHash),
		Raw:   raw,
		Hash:  msgHash,
	}
}

// MakeSignID creates singID
func MakeSignID(msgHash, reqID []byte, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	return crypto.SignID(
		quorumType,
		tmbytes.Reverse(quorumHash),
		tmbytes.Reverse(reqID),
		tmbytes.Reverse(msgHash),
	)
}
