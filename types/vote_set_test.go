package types

import (
	"bytes"
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestVoteSet_AddVote_Good(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 10, 1)

	val0 := privValidators[0]

	val0p, err := val0.GetPubKey(ctx)
	require.NoError(t, err)
	val0Addr := val0p.Address()

	assert.Nil(t, voteSet.GetByAddress(val0Addr))
	assert.False(t, voteSet.BitArray().GetIndex(0))
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")

	vote := &Vote{
		ValidatorAddress: val0Addr,
		ValidatorIndex:   0, // since privValidators are in order
		Height:           height,
		Round:            round,
		Type:             tmproto.PrevoteType,
		Timestamp:        tmtime.Now(),
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	_, err = signAddVote(ctx, val0, vote, voteSet)
	require.NoError(t, err)

	assert.NotNil(t, voteSet.GetByAddress(val0Addr))
	assert.True(t, voteSet.BitArray().GetIndex(0))
	blockID, ok = voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")
}

func TestVoteSet_AddVote_Bad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrevoteType,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		pubKey, err := privValidators[0].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 0)
		added, err := signAddVote(ctx, privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for some block.
	{
		pubKey, err := privValidators[0].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, conflicting vote.")
		}
	}

	// val1 votes on another height
	{
		pubKey, err := privValidators[1].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 1)
		added, err := signAddVote(ctx, privValidators[1], withHeight(vote, height+1), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong height")
		}
	}

	// val2 votes on another round
	{
		pubKey, err := privValidators[2].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 2)
		added, err := signAddVote(ctx, privValidators[2], withRound(vote, round+1), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong round")
		}
	}

	// val3 votes of another type.
	{
		pubKey, err := privValidators[3].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 3)
		added, err := signAddVote(ctx, privValidators[3], withType(vote, byte(tmproto.PrecommitType)), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong type")
		}
	}

}

func TestVoteSet_2_3Majority(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Type:             tmproto.PrevoteType,
		Timestamp:        tmtime.Now(),
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	// 6 out of 10 voted for nil.
	for i := int32(0); i < 6; i++ {
		pubKey, err := privValidators[i].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, i)
		_, err = signAddVote(ctx, privValidators[i], vote, voteSet)
		require.NoError(t, err)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")

	// 7th validator voted for some blockhash
	{
		pubKey, err := privValidators[6].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 6)
		_, err = signAddVote(ctx, privValidators[6], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")
	}

	// 8th validator voted for nil.
	{
		pubKey, err := privValidators[7].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 7)
		_, err = signAddVote(ctx, privValidators[7], vote, voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.True(t, ok || blockID.IsNil(), "there should be 2/3 majority for nil")
	}
}

func TestVoteSet_2_3MajorityRedux(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 100, 1)

	blockHash := crypto.CRandBytes(32)
	blockPartsTotal := uint32(123)
	blockPartSetHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrevoteType,
		BlockID:          BlockID{blockHash, blockPartSetHeader},
	}

	// 66 out of 100 voted for nil.
	for i := int32(0); i < 66; i++ {
		pubKey, err := privValidators[i].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, i)
		_, err = signAddVote(ctx, privValidators[i], vote, voteSet)
		require.NoError(t, err)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(),
		"there should be no 2/3 majority")

	// 67th validator voted for nil
	{
		pubKey, err := privValidators[66].GetPubKey(ctx)
		require.NoError(t, err)
		adrr := pubKey.Address()
		vote := withValidator(voteProto, adrr, 66)
		_, err = signAddVote(ctx, privValidators[66], withBlockHash(vote, nil), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added was nil")
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		pubKey, err := privValidators[67].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 67)
		blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}
		_, err = signAddVote(ctx, privValidators[67], withBlockPartSetHeader(vote, blockPartsHeader), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added had different PartSetHeader Hash")
	}

	// 69th validator voted for different BlockParts Total
	{
		pubKey, err := privValidators[68].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 68)
		blockPartsHeader := PartSetHeader{blockPartsTotal + 1, blockPartSetHeader.Hash}
		_, err = signAddVote(ctx, privValidators[68], withBlockPartSetHeader(vote, blockPartsHeader), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added had different PartSetHeader Total")
	}

	// 70th validator voted for different BlockHash
	{
		pubKey, err := privValidators[69].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 69)
		_, err = signAddVote(ctx, privValidators[69], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added had different BlockHash")
	}

	// 71st validator voted for the right BlockHash & BlockPartSetHeader
	{
		pubKey, err := privValidators[70].GetPubKey(ctx)
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 70)
		_, err = signAddVote(ctx, privValidators[70], vote, voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.True(t, ok && blockID.Equals(BlockID{blockHash, blockPartSetHeader}),
			"there should be 2/3 majority")
	}
}

func TestVoteSet_Conflicts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 4, 1)

	blockHash1 := tmrand.Bytes(32)
	blockHash2 := tmrand.Bytes(32)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrevoteType,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	val0, err := privValidators[0].GetPubKey(ctx)
	require.NoError(t, err)
	val0Addr := val0.Address()

	// val0 votes for nil.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := signAddVote(ctx, privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		assert.False(t, added, "conflicting vote")
		assert.Error(t, err, "conflicting vote")
	}

	// start tracking blockHash1
	err = voteSet.SetPeerMaj23("peerA", BlockID{blockHash1, PartSetHeader{}})
	require.NoError(t, err)

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		assert.True(t, added, "called SetPeerMaj23()")
		assert.Error(t, err, "conflicting vote")
	}

	// attempt tracking blockHash2, should fail because already set for peerA.
	err = voteSet.SetPeerMaj23("peerA", BlockID{blockHash2, PartSetHeader{}})
	require.Error(t, err)

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, blockHash2), voteSet)
		assert.False(t, added, "duplicate SetPeerMaj23() from peerA")
		assert.Error(t, err, "conflicting vote")
	}

	// val1 votes for blockHash1.
	{
		pv, err := privValidators[1].GetPubKey(ctx)
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 1)
		added, err := signAddVote(ctx, privValidators[1], withBlockHash(vote, blockHash1), voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// check
	if voteSet.HasTwoThirdsMajority() {
		t.Errorf("we shouldn't have 2/3 majority yet")
	}
	if voteSet.HasTwoThirdsAny() {
		t.Errorf("we shouldn't have 2/3 if any votes yet")
	}

	// val2 votes for blockHash2.
	{
		pv, err := privValidators[2].GetPubKey(ctx)
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 2)
		added, err := signAddVote(ctx, privValidators[2], withBlockHash(vote, blockHash2), voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// check
	if voteSet.HasTwoThirdsMajority() {
		t.Errorf("we shouldn't have 2/3 majority yet")
	}
	if !voteSet.HasTwoThirdsAny() {
		t.Errorf("we should have 2/3 if any votes")
	}

	// now attempt tracking blockHash1
	err = voteSet.SetPeerMaj23("peerB", BlockID{blockHash1, PartSetHeader{}})
	require.NoError(t, err)

	// val2 votes for blockHash1.
	{
		pv, err := privValidators[2].GetPubKey(ctx)
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 2)
		added, err := signAddVote(ctx, privValidators[2], withBlockHash(vote, blockHash1), voteSet)
		assert.True(t, added)
		assert.Error(t, err, "conflicting vote")
	}

	// check
	if !voteSet.HasTwoThirdsMajority() {
		t.Errorf("we should have 2/3 majority for blockHash1")
	}
	blockIDMaj23, _ := voteSet.TwoThirdsMajority()
	if !bytes.Equal(blockIDMaj23.Hash, blockHash1) {
		t.Errorf("got the wrong 2/3 majority blockhash")
	}
	if !voteSet.HasTwoThirdsAny() {
		t.Errorf("we should have 2/3 if any votes")
	}
}

func TestVoteSet_MakeCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrecommitType, 10, 1)

	blockHash, blockPartSetHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrecommitType,
		BlockID:          BlockID{blockHash, blockPartSetHeader},
	}

	// 6 out of 10 voted for some block.
	for i := int32(0); i < 6; i++ {
		pv, err := privValidators[i].GetPubKey(ctx)
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, i)
		_, err = signAddVote(ctx, privValidators[i], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
	}

	// MakeCommit should fail.
	assert.Panics(t, func() { voteSet.MakeExtendedCommit() }, "Doesn't have +2/3 majority")

	// 7th voted for some other block.
	{
		pv, err := privValidators[6].GetPubKey(ctx)
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 6)
		vote = withBlockHash(vote, tmrand.Bytes(32))
		vote = withBlockPartSetHeader(vote, PartSetHeader{123, tmrand.Bytes(32)})

		_, err = signAddVote(ctx, privValidators[6], vote, voteSet)
		require.NoError(t, err)
	}

	// The 8th voted like everyone else.
	{
		pv, err := privValidators[7].GetPubKey(ctx)
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 7)
		_, err = signAddVote(ctx, privValidators[7], vote, voteSet)
		require.NoError(t, err)
	}

	// The 9th voted for nil.
	{
		pv, err := privValidators[8].GetPubKey(ctx)
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 8)
		vote.BlockID = BlockID{}

		_, err = signAddVote(ctx, privValidators[8], vote, voteSet)
		require.NoError(t, err)
	}

	extCommit := voteSet.MakeExtendedCommit()

	// Commit should have 10 elements
	assert.Equal(t, 10, len(extCommit.ExtendedSignatures))

	// Ensure that Commit is good.
	if err := extCommit.ValidateBasic(); err != nil {
		t.Errorf("error in Commit.ValidateBasic(): %v", err)
	}
}

// TestVoteSet_VoteExtensionsEnabled tests that the vote set correctly validates
// vote extensions data when either required or not required.
func TestVoteSet_VoteExtensionsEnabled(t *testing.T) {
	for _, tc := range []struct {
		name              string
		requireExtensions bool
		addExtension      bool
		exepectError      bool
	}{
		{
			name:              "no extension but expected",
			requireExtensions: true,
			addExtension:      false,
			exepectError:      true,
		},
		{
			name:              "invalid extensions but not expected",
			requireExtensions: true,
			addExtension:      false,
			exepectError:      true,
		},
		{
			name:              "no extension and not expected",
			requireExtensions: false,
			addExtension:      false,
			exepectError:      false,
		},
		{
			name:              "extension and expected",
			requireExtensions: true,
			addExtension:      true,
			exepectError:      false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			height, round := int64(1), int32(0)
			valSet, privValidators := randValidatorPrivValSet(ctx, t, 5, 10)
			var voteSet *VoteSet
			if tc.requireExtensions {
				voteSet = NewExtendedVoteSet("test_chain_id", height, round, tmproto.PrecommitType, valSet)
			} else {
				voteSet = NewVoteSet("test_chain_id", height, round, tmproto.PrecommitType, valSet)
			}

			val0 := privValidators[0]

			val0p, err := val0.GetPubKey(ctx)
			require.NoError(t, err)
			val0Addr := val0p.Address()
			blockHash := crypto.CRandBytes(32)
			blockPartsTotal := uint32(123)
			blockPartSetHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}

			vote := &Vote{
				ValidatorAddress: val0Addr,
				ValidatorIndex:   0,
				Height:           height,
				Round:            round,
				Type:             tmproto.PrecommitType,
				Timestamp:        tmtime.Now(),
				BlockID:          BlockID{blockHash, blockPartSetHeader},
			}
			v := vote.ToProto()
			err = val0.SignVote(ctx, voteSet.ChainID(), v)
			require.NoError(t, err)
			vote.Signature = v.Signature

			if tc.addExtension {
				vote.ExtensionSignature = v.ExtensionSignature
			}

			added, err := voteSet.AddVote(vote)
			if tc.exepectError {
				require.Error(t, err)
				require.False(t, added)
			} else {
				require.NoError(t, err)
				require.True(t, added)
			}
		})
	}
}

// NOTE: privValidators are in order
func randVoteSet(
	ctx context.Context,
	t testing.TB,
	height int64,
	round int32,
	signedMsgType tmproto.SignedMsgType,
	numValidators int,
	votingPower int64,
) (*VoteSet, *ValidatorSet, []PrivValidator) {
	t.Helper()
	valSet, privValidators := randValidatorPrivValSet(ctx, t, numValidators, votingPower)
	return NewExtendedVoteSet("test_chain_id", height, round, signedMsgType, valSet), valSet, privValidators
}

func deterministicVoteSet(
	ctx context.Context,
	t *testing.T,
	height int64,
	round int32,
	signedMsgType tmproto.SignedMsgType,
	votingPower int64,
) (*VoteSet, *ValidatorSet, []PrivValidator) {
	t.Helper()
	valSet, privValidators := deterministicValidatorSet(ctx, t)
	return NewExtendedVoteSet("test_chain_id", height, round, signedMsgType, valSet), valSet, privValidators
}

func randValidatorPrivValSet(ctx context.Context, t testing.TB, numValidators int, votingPower int64) (*ValidatorSet, []PrivValidator) {
	var (
		valz           = make([]*Validator, numValidators)
		privValidators = make([]PrivValidator, numValidators)
	)

	for i := 0; i < numValidators; i++ {
		val, privValidator, err := randValidator(ctx, false, votingPower)
		require.NoError(t, err)

		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(PrivValidatorsByAddress(privValidators))

	return NewValidatorSet(valz), privValidators
}

// Convenience: Return new vote with different validator address/index
func withValidator(vote *Vote, addr []byte, idx int32) *Vote {
	vote = vote.Copy()
	vote.ValidatorAddress = addr
	vote.ValidatorIndex = idx
	return vote
}

// Convenience: Return new vote with different height
func withHeight(vote *Vote, height int64) *Vote {
	vote = vote.Copy()
	vote.Height = height
	return vote
}

// Convenience: Return new vote with different round
func withRound(vote *Vote, round int32) *Vote {
	vote = vote.Copy()
	vote.Round = round
	return vote
}

// Convenience: Return new vote with different type
func withType(vote *Vote, signedMsgType byte) *Vote {
	vote = vote.Copy()
	vote.Type = tmproto.SignedMsgType(signedMsgType)
	return vote
}

// Convenience: Return new vote with different blockHash
func withBlockHash(vote *Vote, blockHash []byte) *Vote {
	vote = vote.Copy()
	vote.BlockID.Hash = blockHash
	return vote
}

// Convenience: Return new vote with different blockParts
func withBlockPartSetHeader(vote *Vote, blockPartsHeader PartSetHeader) *Vote {
	vote = vote.Copy()
	vote.BlockID.PartSetHeader = blockPartsHeader
	return vote
}
