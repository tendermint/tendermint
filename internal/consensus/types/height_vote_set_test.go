package types

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var cfg *config.Config // NOTE: must be reset for each _test.go file

func TestMain(m *testing.M) {
	var err error
	cfg, err = config.ResetTestRoot("consensus_height_vote_set_test")
	if err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(cfg.RootDir)
	os.Exit(code)
}

func TestPeerCatchupRounds(t *testing.T) {
	valSet, privVals := factory.RandValidatorSet(10)

	stateID := types.StateID{}

	hvs := NewHeightVoteSet(cfg.ChainID(), 1, stateID, valSet)

	vote999_0 := makeVoteHR(t, 1, 0, 999, privVals, valSet.QuorumType, valSet.QuorumHash, stateID)
	added, err := hvs.AddVote(vote999_0, "peer1")
	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

	vote1000_0 := makeVoteHR(t, 1, 0, 1000, privVals, valSet.QuorumType, valSet.QuorumHash, stateID)
	added, err = hvs.AddVote(vote1000_0, "peer1")
	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

	vote1001_0 := makeVoteHR(t, 1, 0, 1001, privVals, valSet.QuorumType, valSet.QuorumHash, stateID)
	added, err = hvs.AddVote(vote1001_0, "peer1")
	if err != ErrGotVoteFromUnwantedRound {
		t.Errorf("expected GotVoteFromUnwantedRoundError, but got %v", err)
	}
	if added {
		t.Error("Expected to *not* add vote from peer, too many catchup rounds.")
	}

	added, err = hvs.AddVote(vote1001_0, "peer2")
	if !added || err != nil {
		t.Error("Expected to successfully add vote from another peer")
	}

}

func makeVoteHR(t *testing.T, height int64, valIndex, round int32, privVals []types.PrivValidator,
	quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, stateID types.StateID) *types.Vote {
	privVal := privVals[valIndex]
	proTxHash, err := privVal.GetProTxHash(context.Background())
	require.NoError(t, err)

	randBytes1 := tmrand.Bytes(tmhash.Size)

	vote := &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     valIndex,
		Height:             height,
		Round:              round,
		Type:               tmproto.PrecommitType,
		BlockID:            types.BlockID{Hash: randBytes1, PartSetHeader: types.PartSetHeader{}},
	}
	chainID := cfg.ChainID()

	v := vote.ToProto()
	err = privVal.SignVote(context.Background(), chainID, quorumType, quorumHash, v, stateID, nil)
	if err != nil {
		panic(fmt.Sprintf("Error signing vote: %v", err))
	}

	vote.BlockSignature = v.BlockSignature
	vote.StateSignature = v.StateSignature

	return vote
}
