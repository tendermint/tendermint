package factory

import (
	"context"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func MakeVote(
	ctx context.Context,
	val types.PrivValidator,
	valSet *types.ValidatorSet,
	chainID string,
	valIndex int32,
	height int64,
	round int32,
	step int,
	blockID types.BlockID,
	stateID types.StateID,
) (*types.Vote, error) {
	proTxHash, err := val.GetProTxHash(ctx)
	if err != nil {
		return nil, err
	}

	v := &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     valIndex,
		Height:             height,
		Round:              round,
		Type:               tmproto.SignedMsgType(step),
		BlockID:            blockID,
	}

	vpb := v.ToProto()

	if err := val.SignVote(ctx, chainID, valSet.QuorumType, valSet.QuorumHash, vpb, stateID, nil); err != nil {
		return nil, err
	}

	v.BlockSignature = vpb.BlockSignature
	v.StateSignature = vpb.StateSignature
	v.VoteExtensions.CopySignsFromProto(vpb.VoteExtensions)
	return v, nil
}
