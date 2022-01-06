package factory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// ErrorHandler checks an error and returns true if the error is
// non-nil. Pass an ErrorHandler to a function in a case where you
// might want to use a *testing.T but cannot because one is not
// available (e.g. as is the case in some e2e test fixtures and in
// TestMain.)
type ErrorHandler func(error) bool

func Require(t *testing.T) ErrorHandler {
	return func(err error) bool {
		t.Helper()
		require.NoError(t, err)
		return err != nil
	}
}

func MakeVote(
	ctx context.Context,
	eh ErrorHandler,
	val types.PrivValidator,
	chainID string,
	valIndex int32,
	height int64,
	round int32,
	step int,
	blockID types.BlockID,
	time time.Time,
) *types.Vote {
	pubKey, err := val.GetPubKey(ctx)
	if eh(err) {
		return nil
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
	if eh(val.SignVote(ctx, chainID, vpb)) {
		return nil
	}

	v.Signature = vpb.Signature
	return v
}
