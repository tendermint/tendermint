package types

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestSigsRecoverer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const height = 1000
	stateID := RandStateID().WithHeight(height - 1)
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	testCases := []struct {
		votes []*Vote
	}{
		{
			votes: []*Vote{
				{
					ValidatorProTxHash: crypto.RandProTxHash(),
					Type:               types.PrecommitType,
					BlockID:            blockID,
					VoteExtensions: mockVoteExtensions(t,
						types.VoteExtensionType_DEFAULT, "default",
						types.VoteExtensionType_THRESHOLD_RECOVER, "threshold",
					),
				},
				{
					ValidatorProTxHash: crypto.RandProTxHash(),
					Type:               types.PrecommitType,
					BlockID:            blockID,
					VoteExtensions: mockVoteExtensions(t,
						types.VoteExtensionType_DEFAULT, "default",
						types.VoteExtensionType_THRESHOLD_RECOVER, "threshold",
					),
				},
			},
		},
	}
	chainID := "dash-platform-chain"
	quorumType := crypto.SmallQuorumType()
	quorumHash := crypto.RandQuorumHash()
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
				vote.BlockSignature = protoVote.BlockSignature
				vote.StateSignature = protoVote.StateSignature
				for i, ext := range protoVote.VoteExtensions {
					vote.VoteExtensions[i].Signature = ext.Signature
				}
				pubKey, err := pvs[i].GetPubKey(ctx, quorumHash)
				require.NoError(t, err)
				pubKeys = append(pubKeys, pubKey)
				IDs = append(IDs, vote.ValidatorProTxHash)
			}
			sr := NewSignsRecoverer(tc.votes)
			thresholdSigns, err := sr.Recover()
			require.NoError(t, err)

			signIDs, err := MakeSignIDs(chainID, quorumType, quorumHash, tc.votes[0].ToProto(), stateID)
			require.NoError(t, err)

			thresholdPubKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(pubKeys, IDs)
			require.NoError(t, err)
			verified := thresholdPubKey.VerifySignatureDigest(signIDs.BlockID.ID, thresholdSigns.BlockSign)
			require.True(t, verified)
			verified = thresholdPubKey.VerifySignatureDigest(signIDs.StateID.ID, thresholdSigns.StateSign)
			require.True(t, verified)

			indexes := recoverableVoteExtensionIndexes(tc.votes)
			for i, j := range indexes {
				sig := thresholdSigns.VoteExtSigns[i]
				verified = thresholdPubKey.VerifySignatureDigest(signIDs.VoteExtIDs[j].ID, sig)
				require.True(t, verified)
			}
		})
	}
}

func TestSigsRecoverer_UsingVoteSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		chainID = "dash-platform-chain"
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
			Type:               types.PrecommitType,
			BlockID:            blockID,
			VoteExtensions: mockVoteExtensions(t,
				types.VoteExtensionType_DEFAULT, "default",
				types.VoteExtensionType_THRESHOLD_RECOVER, "threshold",
			),
		}
		vpb := votes[i].ToProto()
		err = pvs[i].SignVote(ctx, chainID, quorumType, quorumHash, vpb, stateID, nil)
		require.NoError(t, err)
		votes[i].BlockSignature = vpb.BlockSignature
		votes[i].StateSignature = vpb.StateSignature
		for j, ext := range vpb.VoteExtensions {
			votes[i].VoteExtensions[j].Signature = ext.Signature
		}
	}
	voteSet := NewVoteSet(chainID, height, 0, types.PrecommitType, vals, stateID)
	for _, vote := range votes {
		added, err := voteSet.AddVote(vote)
		require.NoError(t, err)
		require.True(t, added)
	}
}

func mockVoteExtensions(t *testing.T, pairs ...interface{}) []VoteExtension {
	exts := make([]VoteExtension, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		ext := VoteExtension{
			Type: pairs[i].(types.VoteExtensionType),
		}
		switch v := pairs[i+1].(type) {
		case string:
			ext.Extension = []byte(v)
		case []byte:
			ext.Extension = v
		default:
			t.Fatalf("given unsupported type %T", pairs[i+1])
		}
		exts[i/2] = ext
	}
	return exts
}
