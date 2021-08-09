package consensus_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	test "github.com/tendermint/tendermint/internal/test/factory"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/meta"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestVoteSet_AddVote_Good(t *testing.T) {
	height, round := int64(1), int32(0)
	voteSet, _, privValidators := test.RandVoteSet(height, round, tmproto.PrevoteType, 10, 1)
	val0 := privValidators[0]

	val0p, err := val0.GetPubKey(context.Background())
	require.NoError(t, err)
	val0Addr := val0p.Address()

	assert.Nil(t, voteSet.GetByAddress(val0Addr))
	assert.False(t, voteSet.BitArray().GetIndex(0))
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsZero(), "there should be no 2/3 majority")

	vote := &consensus.Vote{
		ValidatorAddress: val0Addr,
		ValidatorIndex:   0, // since privValidators are in order
		Height:           height,
		Round:            round,
		Type:             tmproto.PrevoteType,
		Timestamp:        tmtime.Now(),
		BlockID:          meta.BlockID{nil, meta.PartSetHeader{}},
	}
	_, err = test.SignAddVote(val0, vote, voteSet)
	require.NoError(t, err)

	assert.NotNil(t, voteSet.GetByAddress(val0Addr))
	assert.True(t, voteSet.BitArray().GetIndex(0))
	blockID, ok = voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsZero(), "there should be no 2/3 majority")
}

func TestVoteSet_AddVote_Bad(t *testing.T) {
	height, round := int64(1), int32(0)
	voteSet, _, privValidators := test.RandVoteSet(height, round, tmproto.PrevoteType, 10, 1)

	voteProto := &consensus.Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrevoteType,
		BlockID:          meta.BlockID{nil, meta.PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		pubKey, err := privValidators[0].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 0)
		added, err := test.SignAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for some meta.
	{
		pubKey, err := privValidators[0].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 0)
		added, err := test.SignAddVote(privValidators[0], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, conflicting vote.")
		}
	}

	// val1 votes on another height
	{
		pubKey, err := privValidators[1].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 1)
		added, err := test.SignAddVote(privValidators[1], withHeight(vote, height+1), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong height")
		}
	}

	// val2 votes on another round
	{
		pubKey, err := privValidators[2].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 2)
		added, err := test.SignAddVote(privValidators[2], withRound(vote, round+1), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong round")
		}
	}

	// val3 votes of another type.
	{
		pubKey, err := privValidators[3].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 3)
		added, err := test.SignAddVote(privValidators[3], withType(vote, byte(tmproto.PrecommitType)), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong type")
		}
	}
}

func TestVoteSet_2_3Majority(t *testing.T) {
	height, round := int64(1), int32(0)
	voteSet, _, privValidators := test.RandVoteSet(height, round, tmproto.PrevoteType, 10, 1)

	voteProto := &consensus.Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Type:             tmproto.PrevoteType,
		Timestamp:        tmtime.Now(),
		BlockID:          meta.BlockID{nil, meta.PartSetHeader{}},
	}
	// 6 out of 10 voted for nil.
	for i := int32(0); i < 6; i++ {
		pubKey, err := privValidators[i].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, i)
		_, err = test.SignAddVote(privValidators[i], vote, voteSet)
		require.NoError(t, err)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsZero(), "there should be no 2/3 majority")

	// 7th validator voted for some blockhash
	{
		pubKey, err := privValidators[6].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 6)
		_, err = test.SignAddVote(privValidators[6], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsZero(), "there should be no 2/3 majority")
	}

	// 8th validator voted for nil.
	{
		pubKey, err := privValidators[7].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 7)
		_, err = test.SignAddVote(privValidators[7], vote, voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.True(t, ok || blockID.IsZero(), "there should be 2/3 majority for nil")
	}
}

func TestVoteSet_2_3MajorityRedux(t *testing.T) {
	height, round := int64(1), int32(0)
	voteSet, _, privValidators := test.RandVoteSet(height, round, tmproto.PrevoteType, 100, 1)

	blockHash := crypto.CRandBytes(32)
	blockPartsTotal := uint32(123)
	blockPartSetHeader := meta.PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}

	voteProto := &consensus.Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrevoteType,
		BlockID:          meta.BlockID{blockHash, blockPartSetHeader},
	}

	// 66 out of 100 voted for nil.
	for i := int32(0); i < 66; i++ {
		pubKey, err := privValidators[i].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, i)
		_, err = test.SignAddVote(privValidators[i], vote, voteSet)
		require.NoError(t, err)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsZero(),
		"there should be no 2/3 majority")

	// 67th validator voted for nil
	{
		pubKey, err := privValidators[66].GetPubKey(context.Background())
		require.NoError(t, err)
		adrr := pubKey.Address()
		vote := withValidator(voteProto, adrr, 66)
		_, err = test.SignAddVote(privValidators[66], withBlockHash(vote, nil), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsZero(),
			"there should be no 2/3 majority: last vote added was nil")
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		pubKey, err := privValidators[67].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 67)
		blockPartsHeader := meta.PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}
		_, err = test.SignAddVote(privValidators[67], withBlockPartSetHeader(vote, blockPartsHeader), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsZero(),
			"there should be no 2/3 majority: last vote added had different PartSetHeader Hash")
	}

	// 69th validator voted for different BlockParts Total
	{
		pubKey, err := privValidators[68].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 68)
		blockPartsHeader := meta.PartSetHeader{blockPartsTotal + 1, blockPartSetHeader.Hash}
		_, err = test.SignAddVote(privValidators[68], withBlockPartSetHeader(vote, blockPartsHeader), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsZero(),
			"there should be no 2/3 majority: last vote added had different PartSetHeader Total")
	}

	// 70th validator voted for different BlockHash
	{
		pubKey, err := privValidators[69].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 69)
		_, err = test.SignAddVote(privValidators[69], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsZero(),
			"there should be no 2/3 majority: last vote added had different BlockHash")
	}

	// 71st validator voted for the right BlockHash & BlockPartSetHeader
	{
		pubKey, err := privValidators[70].GetPubKey(context.Background())
		require.NoError(t, err)
		addr := pubKey.Address()
		vote := withValidator(voteProto, addr, 70)
		_, err = test.SignAddVote(privValidators[70], vote, voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.True(t, ok && blockID.Equals(meta.BlockID{blockHash, blockPartSetHeader}),
			"there should be 2/3 majority")
	}
}

func TestVoteSet_Conflicts(t *testing.T) {
	height, round := int64(1), int32(0)
	voteSet, _, privValidators := test.RandVoteSet(height, round, tmproto.PrevoteType, 4, 1)
	blockHash1 := tmrand.Bytes(32)
	blockHash2 := tmrand.Bytes(32)

	voteProto := &consensus.Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrevoteType,
		BlockID:          meta.BlockID{nil, meta.PartSetHeader{}},
	}

	val0, err := privValidators[0].GetPubKey(context.Background())
	require.NoError(t, err)
	val0Addr := val0.Address()

	// val0 votes for nil.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := test.SignAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := test.SignAddVote(privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		assert.False(t, added, "conflicting vote")
		assert.Error(t, err, "conflicting vote")
	}

	// start tracking blockHash1
	err = voteSet.SetPeerMaj23("peerA", meta.BlockID{blockHash1, meta.PartSetHeader{}})
	require.NoError(t, err)

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := test.SignAddVote(privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		assert.True(t, added, "called SetPeerMaj23()")
		assert.Error(t, err, "conflicting vote")
	}

	// attempt tracking blockHash2, should fail because already set for peerA.
	err = voteSet.SetPeerMaj23("peerA", meta.BlockID{blockHash2, meta.PartSetHeader{}})
	require.Error(t, err)

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := test.SignAddVote(privValidators[0], withBlockHash(vote, blockHash2), voteSet)
		assert.False(t, added, "duplicate SetPeerMaj23() from peerA")
		assert.Error(t, err, "conflicting vote")
	}

	// val1 votes for blockHash1.
	{
		pv, err := privValidators[1].GetPubKey(context.Background())
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 1)
		added, err := test.SignAddVote(privValidators[1], withBlockHash(vote, blockHash1), voteSet)
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
		pv, err := privValidators[2].GetPubKey(context.Background())
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 2)
		added, err := test.SignAddVote(privValidators[2], withBlockHash(vote, blockHash2), voteSet)
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
	err = voteSet.SetPeerMaj23("peerB", meta.BlockID{blockHash1, meta.PartSetHeader{}})
	require.NoError(t, err)

	// val2 votes for blockHash1.
	{
		pv, err := privValidators[2].GetPubKey(context.Background())
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 2)
		added, err := test.SignAddVote(privValidators[2], withBlockHash(vote, blockHash1), voteSet)
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
	height, round := int64(1), int32(0)
	voteSet, _, privValidators := test.RandVoteSet(height, round, tmproto.PrecommitType, 10, 1)
	blockHash, blockPartSetHeader := crypto.CRandBytes(32), meta.PartSetHeader{123, crypto.CRandBytes(32)}

	voteProto := &consensus.Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrecommitType,
		BlockID:          meta.BlockID{blockHash, blockPartSetHeader},
	}

	// 6 out of 10 voted for some meta.
	for i := int32(0); i < 6; i++ {
		pv, err := privValidators[i].GetPubKey(context.Background())
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, i)
		_, err = test.SignAddVote(privValidators[i], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
	}

	// MakeCommit should fail.
	assert.Panics(t, func() { voteSet.MakeCommit() }, "Doesn't have +2/3 majority")

	// 7th voted for some other meta.
	{
		pv, err := privValidators[6].GetPubKey(context.Background())
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 6)
		vote = withBlockHash(vote, tmrand.Bytes(32))
		vote = withBlockPartSetHeader(vote, meta.PartSetHeader{123, tmrand.Bytes(32)})

		_, err = test.SignAddVote(privValidators[6], vote, voteSet)
		require.NoError(t, err)
	}

	// The 8th voted like everyone else.
	{
		pv, err := privValidators[7].GetPubKey(context.Background())
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 7)
		_, err = test.SignAddVote(privValidators[7], vote, voteSet)
		require.NoError(t, err)
	}

	// The 9th voted for nil.
	{
		pv, err := privValidators[8].GetPubKey(context.Background())
		assert.NoError(t, err)
		addr := pv.Address()
		vote := withValidator(voteProto, addr, 8)
		vote.BlockID = meta.BlockID{}

		_, err = test.SignAddVote(privValidators[8], vote, voteSet)
		require.NoError(t, err)
	}

	commit := voteSet.MakeCommit()

	// Commit should have 10 elements
	assert.Equal(t, 10, len(commit.Signatures))

	// Ensure that Commit is good.
	if err := commit.ValidateBasic(); err != nil {
		t.Errorf("error in Commit.ValidateBasic(): %v", err)
	}
}

func TestCommitToVoteSet(t *testing.T) {
	lastID := test.MakeBlockID()
	h := int64(3)

	voteSet, valSet, vals := test.RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := test.MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	assert.NoError(t, err)

	chainID := voteSet.ChainID()
	voteSet2 := consensus.VoteSetFromCommit(chainID, commit, valSet)

	for i := int32(0); int(i) < len(vals); i++ {
		vote1 := voteSet.GetByIndex(i)
		vote2 := voteSet2.GetByIndex(i)
		vote3 := consensus.GetVoteFromCommit(commit, i)

		vote1bz, err := vote1.ToProto().Marshal()
		require.NoError(t, err)
		vote2bz, err := vote2.ToProto().Marshal()
		require.NoError(t, err)
		vote3bz, err := vote3.ToProto().Marshal()
		require.NoError(t, err)
		assert.Equal(t, vote1bz, vote2bz)
		assert.Equal(t, vote1bz, vote3bz)
	}
}

func TestCommitToVoteSetWithVotesForNilBlock(t *testing.T) {
	blockID := test.MakeBlockIDWithHash([]byte("blockhash"))

	const (
		height = int64(3)
		round  = 0
	)

	type commitVoteTest struct {
		blockIDs      []meta.BlockID
		numVotes      []int // must sum to numValidators
		numValidators int
		valid         bool
	}

	testCases := []commitVoteTest{
		{[]meta.BlockID{blockID, {}}, []int{67, 33}, 100, true},
	}

	for _, tc := range testCases {
		voteSet, valSet, vals := test.RandVoteSet(height-1, round, tmproto.PrecommitType, tc.numValidators, 1)

		vi := int32(0)
		for n := range tc.blockIDs {
			for i := 0; i < tc.numVotes[n]; i++ {
				pubKey, err := vals[vi].GetPubKey(context.Background())
				require.NoError(t, err)
				vote := &consensus.Vote{
					ValidatorAddress: pubKey.Address(),
					ValidatorIndex:   vi,
					Height:           height - 1,
					Round:            round,
					Type:             tmproto.PrecommitType,
					BlockID:          tc.blockIDs[n],
					Timestamp:        tmtime.Now(),
				}

				added, err := test.SignAddVote(vals[vi], vote, voteSet)
				assert.NoError(t, err)
				assert.True(t, added)

				vi++
			}
		}

		if tc.valid {
			commit := voteSet.MakeCommit() // panics without > 2/3 valid votes
			assert.NotNil(t, commit)
			err := valSet.VerifyCommit(voteSet.ChainID(), blockID, height-1, commit)
			assert.Nil(t, err)
		} else {
			assert.Panics(t, func() { voteSet.MakeCommit() })
		}
	}
}

// Convenience: Return new vote with different validator address/index
func withValidator(vote *consensus.Vote, addr []byte, idx int32) *consensus.Vote {
	vote = vote.Copy()
	vote.ValidatorAddress = addr
	vote.ValidatorIndex = idx
	return vote
}

// Convenience: Return new vote with different height
func withHeight(vote *consensus.Vote, height int64) *consensus.Vote {
	vote = vote.Copy()
	vote.Height = height
	return vote
}

// Convenience: Return new vote with different round
func withRound(vote *consensus.Vote, round int32) *consensus.Vote {
	vote = vote.Copy()
	vote.Round = round
	return vote
}

// Convenience: Return new vote with different type
func withType(vote *consensus.Vote, signedMsgType byte) *consensus.Vote {
	vote = vote.Copy()
	vote.Type = tmproto.SignedMsgType(signedMsgType)
	return vote
}

// Convenience: Return new vote with different blockHash
func withBlockHash(vote *consensus.Vote, blockHash []byte) *consensus.Vote {
	vote = vote.Copy()
	vote.BlockID.Hash = blockHash
	return vote
}

// Convenience: Return new vote with different blockParts
func withBlockPartSetHeader(vote *consensus.Vote, blockPartsHeader meta.PartSetHeader) *consensus.Vote {
	vote = vote.Copy()
	vote.BlockID.PartSetHeader = blockPartsHeader
	return vote
}
