package consensus_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	test "github.com/tendermint/tendermint/internal/test/factory"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/pkg/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

//-------------------------------------
// Benchmark tests
//
func BenchmarkUpdates(b *testing.B) {
	const (
		n = 100
		m = 2000
	)
	// Init with n validators
	vs := make([]*consensus.Validator, n)
	for j := 0; j < n; j++ {
		vs[j] = consensus.NewValidator(ed25519.GenPrivKey().PubKey(), 100)
	}
	valSet := consensus.NewValidatorSet(vs)

	// Make m new validators
	newValList := make([]*consensus.Validator, m)
	for j := 0; j < m; j++ {
		newValList[j] = consensus.NewValidator(ed25519.GenPrivKey().PubKey(), 1000)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Add m validators to valSetCopy
		valSetCopy := valSet.Copy()
		assert.NoError(b, valSetCopy.UpdateWithChangeSet(newValList))
	}
}

func BenchmarkValidatorSet_VerifyCommit_Ed25519(b *testing.B) {
	for _, n := range []int{1, 8, 64, 1024} {
		n := n
		var (
			chainID = "test_chain_id"
			h       = int64(3)
			blockID = test.MakeBlockID()
		)
		b.Run(fmt.Sprintf("valset size %d", n), func(b *testing.B) {
			b.ReportAllocs()
			// generate n validators
			voteSet, valSet, vals := test.RandVoteSet(h, 0, tmproto.PrecommitType, n, int64(n*5))
			// create a commit with n validators
			commit, err := test.MakeCommit(blockID, h, 0, voteSet, vals, time.Now())
			require.NoError(b, err)

			for i := 0; i < b.N/n; i++ {
				err = valSet.VerifyCommit(chainID, blockID, h, commit)
				assert.NoError(b, err)
			}
		})
	}
}

func BenchmarkValidatorSet_VerifyCommitLight_Ed25519(b *testing.B) {
	for _, n := range []int{1, 8, 64, 1024} {
		n := n
		var (
			chainID = "test_chain_id"
			h       = int64(3)
			blockID = test.MakeBlockID()
		)
		b.Run(fmt.Sprintf("valset size %d", n), func(b *testing.B) {
			b.ReportAllocs()
			// generate n validators
			voteSet, valSet, vals := test.RandVoteSet(h, 0, tmproto.PrecommitType, n, int64(n*5))
			// create a commit with n validators
			commit, err := test.MakeCommit(blockID, h, 0, voteSet, vals, time.Now())
			require.NoError(b, err)

			for i := 0; i < b.N/n; i++ {
				err = valSet.VerifyCommitLight(chainID, blockID, h, commit)
				assert.NoError(b, err)
			}
		})
	}
}

func BenchmarkValidatorSet_VerifyCommitLightTrusting_Ed25519(b *testing.B) {
	for _, n := range []int{1, 8, 64, 1024} {
		n := n
		var (
			chainID = "test_chain_id"
			h       = int64(3)
			blockID = test.MakeBlockID()
		)
		b.Run(fmt.Sprintf("valset size %d", n), func(b *testing.B) {
			b.ReportAllocs()
			// generate n validators
			voteSet, valSet, vals := test.RandVoteSet(h, 0, tmproto.PrecommitType, n, int64(n*5))
			// create a commit with n validators
			commit, err := test.MakeCommit(blockID, h, 0, voteSet, vals, time.Now())
			require.NoError(b, err)

			for i := 0; i < b.N/n; i++ {
				err = valSet.VerifyCommitLightTrusting(chainID, commit, tmmath.Fraction{Numerator: 1, Denominator: 3})
				assert.NoError(b, err)
			}
		})
	}
}

func TestValidatorSetProtoBuf(t *testing.T) {
	valset, _ := test.RandValidatorPrivValSet(10, 100)
	valset2, _ := test.RandValidatorPrivValSet(10, 100)
	valset2.Validators[0] = &consensus.Validator{}

	valset3, _ := test.RandValidatorPrivValSet(10, 100)
	valset3.Proposer = nil

	valset4, _ := test.RandValidatorPrivValSet(10, 100)
	valset4.Proposer = &consensus.Validator{}

	testCases := []struct {
		msg      string
		v1       *consensus.ValidatorSet
		expPass1 bool
		expPass2 bool
	}{
		{"success", valset, true, true},
		{"fail valSet2, pubkey empty", valset2, false, false},
		{"fail nil Proposer", valset3, false, false},
		{"fail empty Proposer", valset4, false, false},
		{"fail empty valSet", &consensus.ValidatorSet{}, true, false},
		{"false nil", nil, true, false},
	}
	for _, tc := range testCases {
		protoValSet, err := tc.v1.ToProto()
		if tc.expPass1 {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		valSet, err := consensus.ValidatorSetFromProto(protoValSet)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.EqualValues(t, tc.v1, valSet, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}
