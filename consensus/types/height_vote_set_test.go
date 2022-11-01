package types

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/test"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var config *cfg.Config // NOTE: must be reset for each _test.go file

func TestMain(m *testing.M) {
	config = test.ResetTestRoot("consensus_height_vote_set_test")
	code := m.Run()
	os.RemoveAll(config.RootDir)
	os.Exit(code)
}

func TestPeerCatchupRounds(t *testing.T) {
	valSet, privVals := types.RandValidatorSet(10, 1)

	hvs := NewHeightVoteSet(test.DefaultTestChainID, 1, valSet)

	vote999_0 := makeVoteHR(t, 1, 0, 999, privVals)
	added, err := hvs.AddVote(vote999_0, "peer1")
	require.NoError(t, err)
	require.True(t, added)

	vote1000_0 := makeVoteHR(t, 1, 0, 1000, privVals)
	added, err = hvs.AddVote(vote1000_0, "peer1")
	require.NoError(t, err)
	require.True(t, added)

	vote1001_0 := makeVoteHR(t, 1, 0, 1001, privVals)
	added, err = hvs.AddVote(vote1001_0, "peer1")
	require.Error(t, err)
	require.Equal(t, ErrGotVoteFromUnwantedRound, err)
	require.False(t, added)

	added, err = hvs.AddVote(vote1001_0, "peer2")
	require.NoError(t, err)
	require.True(t, added)
}

func makeVoteHR(t *testing.T, height int64, valIndex, round int32, privVals []types.PrivValidator) *types.Vote {
	privVal := privVals[valIndex]
	pubKey, err := privVal.GetPubKey()
	if err != nil {
		panic(err)
	}

	randBytes := tmrand.Bytes(tmhash.Size)

	vote := &types.Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrecommitType,
		BlockID:          types.BlockID{Hash: randBytes, PartSetHeader: types.PartSetHeader{}},
	}

	v := vote.ToProto()
	err = privVal.SignVote(test.DefaultTestChainID, v)
	if err != nil {
		panic(fmt.Sprintf("Error signing vote: %v", err))
	}

	vote.Signature = v.Signature

	return vote
}
