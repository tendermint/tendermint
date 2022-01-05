package factory

import (
	"context"
	"time"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func MakeCommit(ctx context.Context, eh ErrorHandler, blockID types.BlockID, height int64, round int32, voteSet *types.VoteSet, validators []types.PrivValidator, now time.Time) *types.Commit {
	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey(ctx)
		eh(err)
		vote := &types.Vote{
			ValidatorAddress: pubKey.Address(),
			ValidatorIndex:   int32(i),
			Height:           height,
			Round:            round,
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		v := vote.ToProto()

		eh(validators[i].SignVote(ctx, voteSet.ChainID(), v))
		vote.Signature = v.Signature
		_, err = voteSet.AddVote(vote)
		eh(err)
	}

	return voteSet.MakeCommit()
}
