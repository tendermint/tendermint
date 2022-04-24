package types

import (
	"context"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Check VerifyCommit
// verification.
func TestValidatorSet_VerifyCommit_All(t *testing.T) {
	var (
		proTxHash  = crypto.RandProTxHash()
		privKey    = bls12381.GenPrivKey()
		pubKey     = privKey.PubKey()
		v1         = NewValidatorDefaultVotingPower(pubKey, proTxHash)
		quorumHash = crypto.RandQuorumHash()
		vset       = NewValidatorSet([]*Validator{v1}, v1.PubKey, btcjson.LLMQType_5_60, quorumHash, true)

		chainID = "Lalande21185"
	)

	vote := examplePrecommit(t)
	vote.ValidatorProTxHash = proTxHash
	v := vote.ToProto()

	stateID := RandStateID().WithHeight(v.Height - 1)

	blockSig, err := privKey.SignDigest(VoteBlockSignID(chainID, v, btcjson.LLMQType_5_60, quorumHash))
	require.NoError(t, err)
	stateSig, err := privKey.SignDigest(stateID.SignID(chainID, btcjson.LLMQType_5_60, quorumHash))
	require.NoError(t, err)
	vote.BlockSignature = blockSig
	vote.StateSignature = stateSig

	commit := NewCommit(vote.Height,
		vote.Round,
		vote.BlockID,
		stateID,
		quorumHash,
		vote.BlockSignature,
		vote.StateSignature,
	)

	vote2 := *vote
	blockSig2, err := privKey.SignDigest(VoteBlockSignBytes("EpsilonEridani", v))
	require.NoError(t, err)
	stateSig2, err := privKey.SignDigest(stateID.SignBytes("EpsilonEridani"))
	require.NoError(t, err)
	vote2.BlockSignature = blockSig2
	vote2.StateSignature = stateSig2

	testCases := []struct {
		description string
		chainID     string
		blockID     BlockID
		stateID     StateID
		height      int64
		commit      *Commit
		expErr      bool
	}{
		{"good", chainID, vote.BlockID, stateID, vote.Height, commit, false},

		{"incorrect threshold block signature", "EpsilonEridani", vote.BlockID, stateID, vote.Height, commit, true},
		{"wrong block ID", chainID, makeBlockIDRandom(), stateID, vote.Height, commit, true},
		{"wrong height", chainID, vote.BlockID, stateID, vote.Height - 1, commit, true},

		{"incorrect threshold block signature", chainID, vote.BlockID, stateID, vote.Height,
			NewCommit(vote.Height, vote.Round, vote.BlockID, stateID, quorumHash, nil, nil), true},

		{"incorrect threshold state signature", chainID, vote.BlockID, stateID, vote.Height,
			NewCommit(vote.Height, vote.Round, vote.BlockID, stateID,
				quorumHash, vote.BlockSignature, nil), true},

		{"incorrect threshold block signature", chainID, vote.BlockID, stateID, vote.Height,
			NewCommit(vote.Height, vote.Round, vote.BlockID, stateID, quorumHash, vote2.BlockSignature, vote2.StateSignature), true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			err := vset.VerifyCommit(tc.chainID, tc.blockID, tc.stateID, tc.height, tc.commit)
			if tc.expErr {
				if assert.Error(t, err, "VerifyCommit") {
					assert.Contains(t, err.Error(), tc.description, "VerifyCommit")
				}
			} else {
				assert.NoError(t, err, "VerifyCommit")
			}
		})
	}
}

//-------------------------------------------------------------------

func TestValidatorSet_VerifyCommit_CheckThresholdSignatures(t *testing.T) {
	var (
		chainID = "test_chain_id"
		h       = int64(3)
		blockID = makeBlockIDRandom()
		stateID = RandStateID().WithHeight(h - 1)
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	voteSet, valSet, vals := randVoteSet(ctx, t, h, 0, tmproto.PrecommitType, 4, stateID)
	commit, err := makeCommit(ctx, blockID, stateID, h, 0, voteSet, vals)
	require.NoError(t, err)

	// malleate threshold sigs signature
	vote := voteSet.GetByIndex(3)
	v := vote.ToProto()
	err = vals[3].SignVote(context.Background(), "CentaurusA", valSet.QuorumType, valSet.QuorumHash, v, stateID, nil)
	require.NoError(t, err)
	commit.ThresholdBlockSignature = v.BlockSignature
	commit.ThresholdStateSignature = v.StateSignature

	err = valSet.VerifyCommit(chainID, blockID, stateID, h, commit)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "incorrect threshold block signature")
	}
}
