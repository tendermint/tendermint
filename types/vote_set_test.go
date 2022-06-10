package types

import (
	"bytes"
	"context"
	"math"
	"sort"
	"strconv"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/llmq"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestVoteSet_AddVote_Good(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 10,
		RandStateID().WithHeight(height-1))
	val0 := privValidators[0]

	val0ProTxHash, err := val0.GetProTxHash(ctx)
	require.NoError(t, err)

	assert.Nil(t, voteSet.GetByProTxHash(val0ProTxHash))
	assert.False(t, voteSet.BitArray().GetIndex(0))
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")

	vote := &Vote{
		ValidatorProTxHash: val0ProTxHash,
		ValidatorIndex:     0, // since privValidators are in order
		Height:             height,
		Round:              round,
		Type:               tmproto.PrevoteType,
		BlockID:            BlockID{nil, PartSetHeader{}},
	}
	_, err = signAddVote(ctx, val0, vote, voteSet)
	require.NoError(t, err)

	assert.NotNil(t, voteSet.GetByProTxHash(val0ProTxHash))
	assert.True(t, voteSet.BitArray().GetIndex(0))
	blockID, ok = voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")
}

func TestVoteSet_AddVote_Bad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 10,
		RandStateID().WithHeight(height-1))

	voteProto := &Vote{
		ValidatorProTxHash: nil,
		ValidatorIndex:     -1,
		Height:             height,
		Round:              round,
		Type:               tmproto.PrevoteType,
		BlockID:            BlockID{nil, PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		proTxHash, err := privValidators[0].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 0)
		added, err := signAddVote(ctx, privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for some block.
	{
		proTxHash, err := privValidators[0].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, conflicting vote.")
		}
	}

	// val1 votes on another height
	{
		proTxHash, err := privValidators[1].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 1)
		added, err := signAddVote(ctx, privValidators[1], withHeight(vote, height+1), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong height")
		}
	}

	// val2 votes on another round
	{
		proTxHash, err := privValidators[2].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 2)
		added, err := signAddVote(ctx, privValidators[2], withRound(vote, round+1), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong round")
		}
	}

	// val3 votes of another type.
	{
		proTxHash, err := privValidators[3].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 3)
		added, err := signAddVote(ctx, privValidators[3], withType(vote, byte(tmproto.PrecommitType)), voteSet)
		if added || err == nil {
			t.Errorf("expected VoteSet.Add to fail, wrong type")
		}
	}

}

// TestVoteSet_AddVote_StateID checks if state signature is verified correctly when adding votes to voteSet
func TestVoteSet_AddVote_StateID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(10), int32(0)

	randStateID1 := RandStateID().WithHeight(height - 1)
	randStateID2 := RandStateID().WithHeight(height - 1)

	testCases := []struct {
		name           string
		voteSetStateID StateID
		wrongStateID   StateID
		shouldFail     bool
	}{
		{"correct", randStateID1, randStateID1, false},
		{"wrong apphash", randStateID1, randStateID2, true},
		{"too low height", randStateID1, randStateID1.WithHeight(height - 5), true},
		{"too high height", randStateID1, randStateID1.WithHeight(height + 5), true},
	}
	//nolint:scopelint
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 10,
				tc.voteSetStateID)

			val0 := privValidators[0]
			val0ProTxHash, err := val0.GetProTxHash(ctx)
			require.NoError(t, err)

			val1 := privValidators[1]
			val1ProTxHash, err := val1.GetProTxHash(ctx)
			require.NoError(t, err)

			assert.Nil(t, voteSet.GetByProTxHash(val0ProTxHash))
			assert.False(t, voteSet.BitArray().GetIndex(0))
			majorityBlockID, ok := voteSet.TwoThirdsMajority()
			assert.False(t, ok || !majorityBlockID.IsNil(), "there should be no 2/3 majority")
			blockID := randBlockID()
			vote1 := &Vote{
				ValidatorProTxHash: val0ProTxHash,
				ValidatorIndex:     0, // since privValidators are in order
				Height:             height,
				Round:              round,
				Type:               tmproto.PrevoteType,
				BlockID:            blockID,
			}
			_, err = signAddVote(ctx, val0, vote1, voteSet)
			require.NoError(t, err)

			vote2 := &Vote{
				ValidatorProTxHash: val1ProTxHash,
				ValidatorIndex:     1, // since privValidators are in order
				Height:             height,
				Round:              round,
				Type:               tmproto.PrevoteType,
				BlockID:            blockID,
			}
			_, err = signAddVoteForStateID(ctx, val1, vote2, voteSet, tc.wrongStateID)
			if tc.shouldFail {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid state signature")
			} else {
				require.NoError(t, err)
			}

			assert.NotNil(t, voteSet.GetByProTxHash(val0ProTxHash))
			assert.True(t, voteSet.BitArray().GetIndex(0))
			majorityBlockID, ok = voteSet.TwoThirdsMajority()
			assert.False(t, ok || !majorityBlockID.IsNil(), "there should be no 2/3 majority")
		})
	}
}

func TestVoteSet_2_3Majority(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height, round := int64(1), int32(0)
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 10,
		RandStateID().WithHeight(height-1))

	voteProto := &Vote{
		ValidatorProTxHash: nil, // NOTE: must fill in
		ValidatorIndex:     -1,  // NOTE: must fill in
		Height:             height,
		Round:              round,
		Type:               tmproto.PrevoteType,
		BlockID:            BlockID{nil, PartSetHeader{}},
	}
	// 6 out of 10 voted for nil.
	for i := int32(0); i < 6; i++ {
		proTxHash, err := privValidators[i].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, i)
		_, err = signAddVote(ctx, privValidators[i], vote, voteSet)
		require.NoError(t, err)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")

	// 7th validator voted for some blockhash
	{
		proTxHash, err := privValidators[6].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 6)
		_, err = signAddVote(ctx, privValidators[6], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(), "there should be no 2/3 majority")
	}

	// 8th validator voted for nil.
	{
		proTxHash, err := privValidators[7].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 7)
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
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 100,
		RandStateID().WithHeight(height-1))

	blockHash := crypto.CRandBytes(32)
	blockPartsTotal := uint32(123)
	blockPartSetHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorProTxHash: nil, // NOTE: must fill in
		ValidatorIndex:     -1,  // NOTE: must fill in
		Height:             height,
		Round:              round,
		Type:               tmproto.PrevoteType,
		BlockID:            BlockID{blockHash, blockPartSetHeader},
	}

	// 66 out of 100 voted for nil.
	for i := int32(0); i < 66; i++ {
		proTxHash, err := privValidators[i].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, i)
		_, err = signAddVote(ctx, privValidators[i], vote, voteSet)
		require.NoError(t, err)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	assert.False(t, ok || !blockID.IsNil(),
		"there should be no 2/3 majority")

	// 67th validator voted for nil
	{
		proTxHash, err := privValidators[66].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 66)
		_, err = signAddVote(ctx, privValidators[66], withBlockHash(vote, nil), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added was nil")
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		proTxHash, err := privValidators[67].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 67)
		blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}
		_, err = signAddVote(ctx, privValidators[67], withBlockPartSetHeader(vote, blockPartsHeader), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added had different PartSetHeader Hash")
	}

	// 69th validator voted for different BlockParts Total
	{
		proTxHash, err := privValidators[68].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 68)
		blockPartsHeader := PartSetHeader{blockPartsTotal + 1, blockPartSetHeader.Hash}
		_, err = signAddVote(ctx, privValidators[68], withBlockPartSetHeader(vote, blockPartsHeader), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added had different PartSetHeader Total")
	}

	// 70th validator voted for different CoreBlockHash
	{
		proTxHash, err := privValidators[69].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 69)
		_, err = signAddVote(ctx, privValidators[69], withBlockHash(vote, tmrand.Bytes(32)), voteSet)
		require.NoError(t, err)
		blockID, ok = voteSet.TwoThirdsMajority()
		assert.False(t, ok || !blockID.IsNil(),
			"there should be no 2/3 majority: last vote added had different CoreBlockHash")
	}

	// 71st validator voted for the right CoreBlockHash & BlockPartSetHeader
	{
		proTxHash, err := privValidators[70].GetProTxHash(ctx)
		require.NoError(t, err)
		vote := withValidator(voteProto, proTxHash, 70)
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
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrevoteType, 4,
		RandStateID().WithHeight(height-1))
	blockHash1 := tmrand.Bytes(32)
	blockHash2 := tmrand.Bytes(32)

	voteProto := &Vote{
		ValidatorProTxHash: nil,
		ValidatorIndex:     -1,
		Height:             height,
		Round:              round,
		Type:               tmproto.PrevoteType,
		BlockID:            BlockID{nil, PartSetHeader{}},
	}

	val0ProTxHash, err := privValidators[0].GetProTxHash(ctx)
	require.NoError(t, err)

	// val0 votes for nil.
	{
		vote := withValidator(voteProto, val0ProTxHash, 0)
		added, err := signAddVote(ctx, privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0ProTxHash, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		assert.False(t, added, "conflicting vote")
		assert.Error(t, err, "conflicting vote")
	}

	// start tracking blockHash1
	err = voteSet.SetPeerMaj23("peerA", BlockID{blockHash1, PartSetHeader{}})
	require.NoError(t, err)

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0ProTxHash, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		assert.True(t, added, "called SetPeerMaj23()")
		assert.Error(t, err, "conflicting vote")
	}

	// attempt tracking blockHash2, should fail because already set for peerA.
	err = voteSet.SetPeerMaj23("peerA", BlockID{blockHash2, PartSetHeader{}})
	require.Error(t, err)

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0ProTxHash, 0)
		added, err := signAddVote(ctx, privValidators[0], withBlockHash(vote, blockHash2), voteSet)
		assert.False(t, added, "duplicate SetPeerMaj23() from peerA")
		assert.Error(t, err, "conflicting vote")
	}

	// val1 votes for blockHash1.
	{
		pvProTxHash, err := privValidators[1].GetProTxHash(ctx)
		assert.NoError(t, err)
		vote := withValidator(voteProto, pvProTxHash, 1)
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
		pvProTxHash, err := privValidators[2].GetProTxHash(context.Background())
		assert.NoError(t, err)
		vote := withValidator(voteProto, pvProTxHash, 2)
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
		pvProTxHash, err := privValidators[2].GetProTxHash(ctx)
		assert.NoError(t, err)
		vote := withValidator(voteProto, pvProTxHash, 2)
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
	voteSet, _, privValidators := randVoteSet(ctx, t, height, round, tmproto.PrecommitType, 10,
		RandStateID().WithHeight(height-1))
	blockHash, blockPartSetHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorProTxHash: nil,
		ValidatorIndex:     -1,
		Height:             height,
		Round:              round,
		Type:               tmproto.PrecommitType,
		BlockID:            BlockID{blockHash, blockPartSetHeader},
	}

	// 6 out of 10 voted for some block.
	for i := int32(0); i < 6; i++ {
		pvProTxHash, err := privValidators[i].GetProTxHash(ctx)
		assert.NoError(t, err)
		vote := withValidator(voteProto, pvProTxHash, i)
		_, err = signAddVote(ctx, privValidators[i], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
	}

	// MakeCommit should fail.
	assert.Panics(t, func() { voteSet.MakeCommit() }, "Doesn't have +2/3 majority")

	// 7th voted for some other block.
	{
		pvProTxHash, err := privValidators[6].GetProTxHash(ctx)
		assert.NoError(t, err)
		vote := withValidator(voteProto, pvProTxHash, 6)
		vote = withBlockHash(vote, tmrand.Bytes(32))
		vote = withBlockPartSetHeader(vote, PartSetHeader{123, tmrand.Bytes(32)})

		_, err = signAddVote(ctx, privValidators[6], vote, voteSet)
		require.NoError(t, err)
	}

	// The 8th voted like everyone else.
	{
		pvProTxHash, err := privValidators[7].GetProTxHash(ctx)
		assert.NoError(t, err)
		vote := withValidator(voteProto, pvProTxHash, 7)
		_, err = signAddVote(ctx, privValidators[7], vote, voteSet)
		require.NoError(t, err)
	}

	// The 9th voted for nil.
	{
		pvProTxHash, err := privValidators[8].GetProTxHash(ctx)
		assert.NoError(t, err)
		vote := withValidator(voteProto, pvProTxHash, 8)
		vote.BlockID = BlockID{}

		_, err = signAddVote(ctx, privValidators[8], vote, voteSet)
		require.NoError(t, err)
	}

	commit := voteSet.MakeCommit()

	// Ensure that Commit is good.
	if err := commit.ValidateBasic(); err != nil {
		t.Errorf("error in Commit.ValidateBasic(): %v", err)
	}
}

func TestVoteSet_LLMQType_50_60(t *testing.T) {
	const (
		height = int64(1)
		round  = int32(0)
	)
	testCases := []struct {
		llmqType      btcjson.LLMQType
		numValidators int
		threshold     int
	}{
		{
			llmqType:      btcjson.LLMQType(0), // "tendermint" algorithm
			numValidators: 40,
			threshold:     int(math.Floor(2.0/3.0*40)) + 1,
		},
		{
			llmqType:      btcjson.LLMQType_50_60,
			numValidators: 35,
			threshold:     30,
		},
		{
			llmqType:      btcjson.LLMQType(0),
			numValidators: 50,
			threshold:     34,
		},
		{
			llmqType:      btcjson.LLMQType_50_60,
			numValidators: 50,
			threshold:     30,
		},
	}

	for ti, tt := range testCases {
		name := strconv.Itoa(ti)
		t.Run(name, func(t *testing.T) {
			voteSet, valSet, privValidators := randVoteSetWithLLMQType(
				height,
				round,
				tmproto.PrevoteType,
				tt.numValidators,
				RandStateID().WithHeight(height-1),
				tt.llmqType,
				tt.threshold,
			)
			assert.EqualValues(t, tt.threshold, valSet.QuorumTypeThresholdCount())
			assert.GreaterOrEqual(t, len(privValidators), tt.threshold+3,
				"need at least %d validators", tt.threshold+3)

			blockHash := crypto.CRandBytes(32)
			blockPartSetHeader := PartSetHeader{uint32(123), crypto.CRandBytes(32)}
			votedBlock := BlockID{blockHash, blockPartSetHeader}

			// below threshold
			for i := 0; i < tt.threshold-1; i++ {
				blockMaj, anyMaj := castVote(t, votedBlock, height, round, privValidators, int32(i), voteSet)
				assert.False(t, blockMaj, "no block majority expected here: i=%d, threshold=%d", i, tt.threshold)
				assert.False(t, anyMaj, "no 'any' majority expected here: i=%d, threshold=%d", i, tt.threshold)
			}

			// we add null vote
			blockMaj, anyMaj := castVote(t, BlockID{}, height, round, privValidators, int32(tt.threshold), voteSet)
			assert.False(t, blockMaj, "no block majority expected after nil vote")
			assert.True(t, anyMaj, "'any' majority expected  after nil vote at threshold")

			// at threshold
			blockMaj, anyMaj = castVote(t, votedBlock, height, round, privValidators, int32(tt.threshold+1), voteSet)
			assert.True(t, blockMaj, "block majority expected")
			assert.True(t, anyMaj, "'any' majority expected")

			// above threshold
			blockMaj, anyMaj = castVote(t, votedBlock, height, round, privValidators, int32(tt.threshold+2), voteSet)
			assert.True(t, blockMaj, "block majority expected")
			assert.True(t, anyMaj, "'any' majority expected")
		})
	}
}

func castVote(
	t *testing.T,
	blockID BlockID,
	height int64,
	round int32,
	privValidators []PrivValidator,
	validatorID int32,
	voteSet *VoteSet,
) (twoThirdsMajority, hasTwoThirdsAny bool) {
	voteProto := &Vote{
		ValidatorProTxHash: nil, // NOTE: must fill in
		ValidatorIndex:     -1,  // NOTE: must fill in
		Height:             height,
		Round:              round,
		Type:               tmproto.PrevoteType,
		BlockID:            blockID,
	}
	ctx := context.Background()
	proTxHash, err := privValidators[validatorID].GetProTxHash(ctx)
	require.NoError(t, err)
	vote := withValidator(voteProto, proTxHash, validatorID)
	signed, err := signAddVote(ctx, privValidators[validatorID], vote, voteSet)
	require.True(t, signed)
	require.NoError(t, err)

	majorityBlock, twoThirdsMajority := voteSet.TwoThirdsMajority()
	assert.EqualValues(t, twoThirdsMajority, !majorityBlock.IsNil())
	return twoThirdsMajority, voteSet.HasTwoThirdsAny()
}

// NOTE: privValidators are in order
func randVoteSet(
	ctx context.Context,
	t testing.TB,
	height int64,
	round int32,
	signedMsgType tmproto.SignedMsgType,
	numValidators int,
	stateID StateID,
) (*VoteSet, *ValidatorSet, []PrivValidator) {
	t.Helper()
	valSet, mockPVs := RandValidatorSet(numValidators)
	return NewVoteSet("test_chain_id", height, round, signedMsgType, valSet, stateID),
		valSet,
		append([]PrivValidator(nil), mockPVs...)
}

func randVoteSetWithLLMQType(
	height int64,
	round int32,
	signedMsgType tmproto.SignedMsgType,
	numValidators int,
	stateID StateID,
	llmqType btcjson.LLMQType,
	threshold int,
) (*VoteSet, *ValidatorSet, []PrivValidator) {
	valz := make([]*Validator, 0, numValidators)
	privValidators := make([]PrivValidator, 0, numValidators)
	ld := llmq.MustGenerate(crypto.RandProTxHashes(numValidators), llmq.WithThreshold(threshold))
	quorumHash := crypto.RandQuorumHash()
	iter := ld.Iter()
	for iter.Next() {
		proTxHash, qks := iter.Value()
		privValidators = append(privValidators, NewMockPVWithParams(
			qks.PrivKey,
			proTxHash,
			quorumHash,
			ld.ThresholdPubKey,
			false,
			false,
		))
		valz = append(valz, NewValidatorDefaultVotingPower(qks.PubKey, proTxHash))
	}

	sort.Sort(PrivValidatorsByProTxHash(privValidators))

	valSet := NewValidatorSet(valz, ld.ThresholdPubKey, llmqType, quorumHash, true)
	voteSet := NewVoteSet("test_chain_id", height, round, signedMsgType,
		valSet, stateID)

	return voteSet, valSet, privValidators
}

// Convenience: Return new vote with different validator address/index
func withValidator(vote *Vote, proTxHash ProTxHash, idx int32) *Vote {
	vote = vote.Copy()
	vote.ValidatorProTxHash = proTxHash
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
