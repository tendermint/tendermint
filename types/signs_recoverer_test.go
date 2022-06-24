package types

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestSigsRecoverer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const (
		height  = 1000
		chainID = "dash-platform"
	)
	stateID := RandStateID().WithHeight(height - 1)
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	quorumType := crypto.SmallQuorumType()
	quorumHash := crypto.RandQuorumHash()
	testCases := []struct {
		votes []*Vote
	}{
		{
			votes: []*Vote{
				{
					ValidatorProTxHash: crypto.RandProTxHash(),
					Type:               tmproto.PrecommitType,
					BlockID:            blockID,
					VoteExtensions: mockVoteExtensions(t,
						tmproto.VoteExtensionType_DEFAULT, "default",
						tmproto.VoteExtensionType_THRESHOLD_RECOVER, "threshold",
					),
				},
				{
					ValidatorProTxHash: crypto.RandProTxHash(),
					Type:               tmproto.PrecommitType,
					BlockID:            blockID,
					VoteExtensions: mockVoteExtensions(t,
						tmproto.VoteExtensionType_DEFAULT, "default",
						tmproto.VoteExtensionType_THRESHOLD_RECOVER, "threshold",
					),
				},
			},
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test-case #%d", i), func(t *testing.T) {
			var (
				pubKeys []crypto.PubKey
				IDs     [][]byte
			)
			pvs := make([]*MockPV, len(tc.votes))
			for i, vote := range tc.votes {
				protoVote := vote.ToProto()
				pvs[i] = NewMockPV(GenKeysForQuorumHash(quorumHash), UseProTxHash(vote.ValidatorProTxHash))
				err := pvs[i].SignVote(ctx, chainID, quorumType, quorumHash, protoVote, stateID, nil)
				require.NoError(t, err)
				err = vote.PopulateSignsFromProto(protoVote)
				require.NoError(t, err)
				pubKey, err := pvs[i].GetPubKey(ctx, quorumHash)
				require.NoError(t, err)
				pubKeys = append(pubKeys, pubKey)
				IDs = append(IDs, vote.ValidatorProTxHash)
			}

			quorumSigns, err := MakeQuorumSigns(chainID, quorumType, quorumHash, tc.votes[0].ToProto(), stateID)
			require.NoError(t, err)

			thresholdPubKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(pubKeys, IDs)
			require.NoError(t, err)

			sr := NewSignsRecoverer(tc.votes)
			thresholdVoteSigns, err := sr.Recover()
			require.NoError(t, err)
			err = quorumSigns.Verify(thresholdPubKey, *thresholdVoteSigns)
			require.NoError(t, err)
		})
	}
}

func TestSigsRecoverer_UsingVoteSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const (
		chainID = "dash-platform"
		height  = 1000
		n       = 4
	)
	stateID := RandStateID().WithHeight(height - 1)
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	vals, pvs := RandValidatorSet(n)
	quorumType := crypto.SmallQuorumType()
	quorumHash, err := pvs[0].GetFirstQuorumHash(ctx)
	require.NoError(t, err)
	votes := make([]*Vote, n)
	for i := 0; i < n; i++ {
		proTxHash, err := pvs[i].GetProTxHash(ctx)
		require.NoError(t, err)
		votes[i] = &Vote{
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     int32(i),
			Height:             height,
			Round:              0,
			Type:               tmproto.PrecommitType,
			BlockID:            blockID,
			VoteExtensions: mockVoteExtensions(t,
				tmproto.VoteExtensionType_DEFAULT, "default",
				tmproto.VoteExtensionType_THRESHOLD_RECOVER, "threshold",
			),
		}
		vpb := votes[i].ToProto()
		err = pvs[i].SignVote(ctx, chainID, quorumType, quorumHash, vpb, stateID, nil)
		require.NoError(t, err)
		err = votes[i].PopulateSignsFromProto(vpb)
		require.NoError(t, err)
	}
	voteSet := NewVoteSet(chainID, height, 0, tmproto.PrecommitType, vals, stateID)
	for _, vote := range votes {
		added, err := voteSet.AddVote(vote)
		require.NoError(t, err)
		require.True(t, added)
	}
}

// mockVoteExtensions returns vote-extensions container, created by passed pairs list
// the format of pairs is
// 1. the length of pairs must be even
// 2. each pair consist of 2 elements: type and extension value
// example: types.VoteExtensionType_DEFAULT, "defailt", types.VoteExtensionType_THRESHOLD_RECOVER, "threshold"
func mockVoteExtensions(t *testing.T, pairs ...interface{}) VoteExtensions {
	if len(pairs)%2 != 0 {
		t.Fatalf("the pairs length must be even")
	}
	ve := make(VoteExtensions)
	for i := 0; i < len(pairs); i += 2 {
		et, ok := pairs[i].(tmproto.VoteExtensionType)
		if !ok {
			t.Fatalf("given unsupported type %T", pairs[i])
		}
		ext := VoteExtension{}
		switch v := pairs[i+1].(type) {
		case string:
			ext.Extension = []byte(v)
		case []byte:
			ext.Extension = v
		default:
			t.Fatalf("given unsupported type %T", pairs[i+1])
		}
		ve[et] = append(ve[et], ext)
	}
	return ve
}
