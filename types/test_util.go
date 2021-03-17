package types

import (
	"context"
	"fmt"
	"time"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func MakeCommit(blockID BlockID, height int64, round int32,
	voteSet *VoteSet, validators []PrivValidator, now time.Time) (*Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey(context.Background())
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

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal PrivValidator, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	v := vote.ToProto()
	err = privVal.SignVote(context.Background(), voteSet.ChainID(), v)
	if err != nil {
		return false, err
	}
	vote.Signature = v.Signature
	return voteSet.AddVote(vote)
}

func MakeVote(
	height int64,
	blockID BlockID,
	valSet *ValidatorSet,
	privVal PrivValidator,
	chainID string,
	now time.Time,
) (*Vote, error) {
	pubKey, err := privVal.GetPubKey(context.Background())
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}
	addr := pubKey.Address()
	idx, _ := valSet.GetByAddress(addr)
	vote := &Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            0,
		Timestamp:        now,
		Type:             tmproto.PrecommitType,
		BlockID:          blockID,
	}
	v := vote.ToProto()

	if err := privVal.SignVote(context.Background(), chainID, v); err != nil {
		return nil, err
	}

	vote.Signature = v.Signature

	return vote, nil
}
