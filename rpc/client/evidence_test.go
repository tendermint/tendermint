package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

func newEvidence(t *testing.T, val *privval.FilePV,
	vote *types.Vote, vote2 *types.Vote,
	chainID string, stateID types.StateID,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	timestamp time.Time,
) *types.DuplicateVoteEvidence {
	t.Helper()
	var err error

	v := vote.ToProto()
	v2 := vote2.ToProto()

	privKey, err := val.GetPrivateKey(context.TODO(), quorumHash)
	require.NoError(t, err)

	vote.BlockSignature, err = privKey.SignDigest(types.VoteBlockSignID(chainID, v, quorumType, quorumHash))
	require.NoError(t, err)

	vote2.BlockSignature, err = privKey.SignDigest(types.VoteBlockSignID(chainID, v2, quorumType, quorumHash))
	require.NoError(t, err)

	vote.StateSignature, err = privKey.SignDigest(stateID.SignID(chainID, quorumType, quorumHash))
	require.NoError(t, err)

	vote2.StateSignature, err = privKey.SignDigest(stateID.SignID(chainID, quorumType, quorumHash))
	require.NoError(t, err)

	validator := types.NewValidator(privKey.PubKey(), types.DefaultDashVotingPower, val.Key.ProTxHash, "")
	valSet := types.NewValidatorSet([]*types.Validator{validator}, validator.PubKey, quorumType, quorumHash, true)

	ev, err := types.NewDuplicateVoteEvidence(vote, vote2, timestamp, valSet)
	require.NoError(t, err)
	return ev
}

func makeEvidences(
	t *testing.T,
	val *privval.FilePV,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	timestamp time.Time,
) (correct *types.DuplicateVoteEvidence, fakes []*types.DuplicateVoteEvidence) {
	vote := types.Vote{
		ValidatorProTxHash: val.Key.ProTxHash,
		ValidatorIndex:     0,
		Height:             1,
		Round:              0,
		Type:               tmproto.PrevoteType,
		BlockID: types.BlockID{
			Hash: crypto.Checksum(tmrand.Bytes(crypto.HashSize)),
			PartSetHeader: types.PartSetHeader{
				Total: 1000,
				Hash:  crypto.Checksum([]byte("partset")),
			},
		},
	}

	stateID := types.RandStateID().WithHeight(vote.Height - 1)

	vote2 := vote
	vote2.BlockID.Hash = crypto.Checksum([]byte("blockhash2"))
	correct = newEvidence(t, val, &vote, &vote2, chainID, stateID, quorumType, quorumHash, timestamp)

	fakes = make([]*types.DuplicateVoteEvidence, 0)

	// different address
	{
		v := vote2
		v.ValidatorProTxHash = []byte("some_pro_tx_hash")
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, stateID, quorumType, quorumHash, timestamp))
	}

	// different height
	{
		v := vote2
		v.Height = vote.Height + 1
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, stateID, quorumType, quorumHash, timestamp))
	}

	// different round
	{
		v := vote2
		v.Round = vote.Round + 1
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, stateID, quorumType, quorumHash, timestamp))
	}

	// different type
	{
		v := vote2
		v.Type = tmproto.PrecommitType
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, stateID, quorumType, quorumHash, timestamp))
	}

	// exactly same vote
	{
		v := vote
		fakes = append(fakes, newEvidence(t, val, &vote, &v, chainID, stateID, quorumType, quorumHash, timestamp))
	}

	return correct, fakes
}

func waitForBlock(ctx context.Context, t *testing.T, c client.Client, height int64) {
	timer := time.NewTimer(0 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			status, err := c.Status(ctx)
			require.NoError(t, err)
			if status.SyncInfo.LatestBlockHeight >= height {
				return
			}
			timer.Reset(200 * time.Millisecond)
		}
	}
}
