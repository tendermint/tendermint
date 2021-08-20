package factory

import (
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func MakeRandomCommit(time time.Time) *metadata.Commit {
	lastID := MakeBlockID()
	h := int64(3)
	voteSet, _, vals := RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals, time)
	if err != nil {
		panic(err)
	}
	return commit
}

func MakeCommit(blockID metadata.BlockID, height int64, round int32,
	voteSet *consensus.VoteSet, validators []consensus.PrivValidator, now time.Time) (*metadata.Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey(context.Background())
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		vote := &consensus.Vote{
			ValidatorAddress: pubKey.Address(),
			ValidatorIndex:   int32(i),
			Height:           height,
			Round:            round,
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		_, err = SignAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func SignAddVote(privVal consensus.PrivValidator, vote *consensus.Vote, voteSet *consensus.VoteSet) (signed bool, err error) {
	v := vote.ToProto()
	err = privVal.SignVote(context.Background(), voteSet.ChainID(), v)
	if err != nil {
		return false, err
	}
	vote.Signature = v.Signature
	return voteSet.AddVote(vote)
}
