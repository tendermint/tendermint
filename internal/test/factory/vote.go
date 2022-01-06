package factory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type ErrorHandler func(error)

func Require(t *testing.T) ErrorHandler {
	return func(err error) {
		t.Helper()
		require.NoError(t, err)
	}
}

func MakeVote(
	ctx context.Context,
	val types.PrivValidator,
	chainID string,
	valIndex int32,
	height int64,
	round int32,
	step int,
	blockID types.BlockID,
	time time.Time,
) (*types.Vote, error) {
	pubKey, err := val.GetPubKey(ctx)
	if err != nil {
		return nil, err
	}
	v := &types.Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             tmproto.SignedMsgType(step),
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	err = val.SignVote(ctx, chainID, vpb)
	if err != nil {
		panic(err)
	}
	v.Signature = vpb.Signature
	return v, nil
}
