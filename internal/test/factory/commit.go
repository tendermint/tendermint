package factory

import (
	"context"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func MakeCommit(
	ctx context.Context,
	blockID types.BlockID,
	height int64,
	round int32,
	voteSet *types.VoteSet,
	validatorSet *types.ValidatorSet,
	validators []types.PrivValidator,
	stateID types.StateID,
) (*types.Commit, error) {
	// all sign
	for i := 0; i < len(validators); i++ {
		proTxHash, err := validators[i].GetProTxHash(ctx)
		if err != nil {
			return nil, err
		}
		vote := &types.Vote{
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     int32(i),
			Height:             height,
			Round:              round,
			Type:               tmproto.PrecommitType,
			BlockID:            blockID,
		}

		v := vote.ToProto()

		if err := validators[i].SignVote(ctx, voteSet.ChainID(), validatorSet.QuorumType, validatorSet.QuorumHash, v, stateID, nil); err != nil {
			return nil, err
		}
		vote.StateSignature = v.StateSignature
		vote.BlockSignature = v.BlockSignature
		for i, ext := range v.VoteExtensions {
			vote.VoteExtensions[i].Signature = ext.Signature
		}
		if _, err := voteSet.AddVote(vote); err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}
