package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
)

func MakeExtCommit(blockID BlockID, height int64, round int32,
	voteSet *VoteSet, validators []PrivValidator, now time.Time) (*ExtendedCommit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey()
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		vote := &Vote{
			ValidatorAddress: pubKey.Address(),
			ValidatorIndex:   int32(i),
			Height:           height,
			Round:            round,
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		_, err = signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeExtendedCommit(), nil
}

// TODO separate and refactor with MakeVote
func signAddVote(privVal PrivValidator, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	if vote.Type != voteSet.signedMsgType {
		return false, fmt.Errorf("vote and voteset are of different types; %d != %d", vote.Type, voteSet.signedMsgType)
	}
	v := vote.ToProto()
	err = privVal.SignVote(voteSet.ChainID(), v)
	if err != nil {
		return false, err
	}
	vote.Signature = v.Signature

	isPrecommit := voteSet.signedMsgType == tmproto.PrecommitType
	if !isPrecommit && voteSet.extensionsEnabled {
		return false, fmt.Errorf("only Precommit vote sets may have extensions enabled; vote type: %d", voteSet.signedMsgType)
	}

	isNil := vote.BlockID.IsZero()
	extSignature := (len(v.ExtensionSignature) > 0)
	if extSignature == (!isPrecommit || isNil) {
		return false, fmt.Errorf(
			"extensions must be present IFF vote is a non-nil Precommit; present %t, vote type %d, is nil %t",
			extSignature,
			voteSet.signedMsgType,
			isNil,
		)
	}

	vote.ExtensionSignature = nil
	if voteSet.extensionsEnabled {
		vote.ExtensionSignature = v.ExtensionSignature
	}

	return voteSet.AddVote(vote)
}

func MakeVoteNoError(
	t *testing.T,
	val PrivValidator,
	chainID string,
	valIndex int32,
	height int64,
	round int32,
	step tmproto.SignedMsgType,
	blockID BlockID,
	time time.Time,
) *Vote {
	vote, err := MakeVote(val, chainID, valIndex, height, round, step, blockID, time)
	require.NoError(t, err)
	return vote
}

func MakeVote(
	val PrivValidator,
	chainID string,
	valIndex int32,
	height int64,
	round int32,
	step tmproto.SignedMsgType,
	blockID BlockID,
	time time.Time,
) (*Vote, error) {
	pubKey, err := val.GetPubKey()
	if err != nil {
		return nil, err
	}

	v := &Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             step,
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	if err := val.SignVote(chainID, vpb); err != nil {
		return nil, err
	}

	v.Signature = vpb.Signature
	if step == tmproto.PrecommitType {
		v.ExtensionSignature = vpb.ExtensionSignature
	}
	return v, nil
}

// func MakeVote(
// 	height int64,
// 	blockID BlockID,
// 	valSet *ValidatorSet,
// 	privVal PrivValidator,
// 	chainID string,
// 	now time.Time,
// ) (*Vote, error) {
// 	pubKey, err := privVal.GetPubKey()
// 	if err != nil {
// 		return nil, fmt.Errorf("can't get pubkey: %w", err)
// 	}
// 	addr := pubKey.Address()
// 	idx, _ := valSet.GetByAddress(addr)

// 	return MakeVote2(privVal, chainID, idx, height, 0, tmproto.PrecommitType, blockID, now)
// }

// MakeBlock returns a new block with an empty header, except what can be
// computed from itself.
// It populates the same set of fields validated by ValidateBasic.
func MakeBlock(height int64, txs []Tx, lastCommit *Commit, evidence []Evidence) *Block {
	block := &Block{
		Header: Header{
			Version: tmversion.Consensus{Block: version.BlockProtocol, App: 0},
			Height:  height,
		},
		Data: Data{
			Txs: txs,
		},
		Evidence:   EvidenceData{Evidence: evidence},
		LastCommit: lastCommit,
	}
	block.fillHeader()
	return block
}
