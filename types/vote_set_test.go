package types

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	tst "github.com/tendermint/tmlibs/test"
)

// NOTE: privValidators are in order
func randVoteSet(height int, round int, type_ byte, numValidators int, votingPower int64) (*VoteSet, *ValidatorSet, []*PrivValidatorFS) {
	valSet, privValidators := RandValidatorSet(numValidators, votingPower)
	return NewVoteSet("test_chain_id", height, round, type_, valSet), valSet, privValidators
}

// Convenience: Return new vote with different validator address/index
func withValidator(vote *Vote, addr []byte, idx int) *Vote {
	vote = vote.Copy()
	vote.ValidatorAddress = addr
	vote.ValidatorIndex = idx
	return vote
}

// Convenience: Return new vote with different height
func withHeight(vote *Vote, height int) *Vote {
	vote = vote.Copy()
	vote.Height = height
	return vote
}

// Convenience: Return new vote with different round
func withRound(vote *Vote, round int) *Vote {
	vote = vote.Copy()
	vote.Round = round
	return vote
}

// Convenience: Return new vote with different type
func withType(vote *Vote, type_ byte) *Vote {
	vote = vote.Copy()
	vote.Type = type_
	return vote
}

// Convenience: Return new vote with different blockHash
func withBlockHash(vote *Vote, blockHash []byte) *Vote {
	vote = vote.Copy()
	vote.BlockID.Hash = blockHash
	return vote
}

// Convenience: Return new vote with different blockParts
func withBlockPartsHeader(vote *Vote, blockPartsHeader PartSetHeader) *Vote {
	vote = vote.Copy()
	vote.BlockID.PartsHeader = blockPartsHeader
	return vote
}

func signAddVote(privVal *PrivValidatorFS, vote *Vote, voteSet *VoteSet) (bool, error) {
	var err error
	vote.Signature, err = privVal.Signer.Sign(SignBytes(voteSet.ChainID(), vote))
	if err != nil {
		return false, err
	}
	added, err := voteSet.AddVote(vote)
	return added, err
}

func TestAddVote(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)
	val0 := privValidators[0]

	// t.Logf(">> %v", voteSet)

	if voteSet.GetByAddress(val0.GetAddress()) != nil {
		t.Errorf("Expected GetByAddress(val0.Address) to be nil")
	}
	if voteSet.BitArray().GetIndex(0) {
		t.Errorf("Expected BitArray.GetIndex(0) to be false")
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	vote := &Vote{
		ValidatorAddress: val0.GetAddress(),
		ValidatorIndex:   0, // since privValidators are in order
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	_, err := signAddVote(val0, vote, voteSet)
	if err != nil {
		t.Error(err)
	}

	if voteSet.GetByAddress(val0.GetAddress()) == nil {
		t.Errorf("Expected GetByAddress(val0.Address) to be present")
	}
	if !voteSet.BitArray().GetIndex(0) {
		t.Errorf("Expected BitArray.GetIndex(0) to be true")
	}
	blockID, ok = voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}
}

func Test2_3Majority(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	// 6 out of 10 voted for nil.
	for i := 0; i < 6; i++ {
		vote := withValidator(voteProto, privValidators[i].GetAddress(), i)
		signAddVote(privValidators[i], vote, voteSet)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	// 7th validator voted for some blockhash
	{
		vote := withValidator(voteProto, privValidators[6].GetAddress(), 6)
		signAddVote(privValidators[6], withBlockHash(vote, cmn.RandBytes(32)), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority")
		}
	}

	// 8th validator voted for nil.
	{
		vote := withValidator(voteProto, privValidators[7].GetAddress(), 7)
		signAddVote(privValidators[7], vote, voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if !ok || !blockID.IsZero() {
			t.Errorf("There should be 2/3 majority for nil")
		}
	}
}

func Test2_3MajorityRedux(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 100, 1)

	blockHash := crypto.CRandBytes(32)
	blockPartsTotal := 123
	blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{blockHash, blockPartsHeader},
	}

	// 66 out of 100 voted for nil.
	for i := 0; i < 66; i++ {
		vote := withValidator(voteProto, privValidators[i].GetAddress(), i)
		signAddVote(privValidators[i], vote, voteSet)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	// 67th validator voted for nil
	{
		vote := withValidator(voteProto, privValidators[66].GetAddress(), 66)
		signAddVote(privValidators[66], withBlockHash(vote, nil), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added was nil")
		}
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		vote := withValidator(voteProto, privValidators[67].GetAddress(), 67)
		blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}
		signAddVote(privValidators[67], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Hash")
		}
	}

	// 69th validator voted for different BlockParts Total
	{
		vote := withValidator(voteProto, privValidators[68].GetAddress(), 68)
		blockPartsHeader := PartSetHeader{blockPartsTotal + 1, blockPartsHeader.Hash}
		signAddVote(privValidators[68], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Total")
		}
	}

	// 70th validator voted for different BlockHash
	{
		vote := withValidator(voteProto, privValidators[69].GetAddress(), 69)
		signAddVote(privValidators[69], withBlockHash(vote, cmn.RandBytes(32)), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different BlockHash")
		}
	}

	// 71st validator voted for the right BlockHash & BlockPartsHeader
	{
		vote := withValidator(voteProto, privValidators[70].GetAddress(), 70)
		signAddVote(privValidators[70], vote, voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if !ok || !blockID.Equals(BlockID{blockHash, blockPartsHeader}) {
			t.Errorf("There should be 2/3 majority")
		}
	}
}

func TestBadVotes(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		vote := withValidator(voteProto, privValidators[0].GetAddress(), 0)
		added, err := signAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for some block.
	{
		vote := withValidator(voteProto, privValidators[0].GetAddress(), 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, cmn.RandBytes(32)), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, conflicting vote.")
		}
	}

	// val1 votes on another height
	{
		vote := withValidator(voteProto, privValidators[1].GetAddress(), 1)
		added, err := signAddVote(privValidators[1], withHeight(vote, height+1), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong height")
		}
	}

	// val2 votes on another round
	{
		vote := withValidator(voteProto, privValidators[2].GetAddress(), 2)
		added, err := signAddVote(privValidators[2], withRound(vote, round+1), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong round")
		}
	}

	// val3 votes of another type.
	{
		vote := withValidator(voteProto, privValidators[3].GetAddress(), 3)
		added, err := signAddVote(privValidators[3], withType(vote, VoteTypePrecommit), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong type")
		}
	}
}

func TestConflicts(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 4, 1)
	blockHash1 := cmn.RandBytes(32)
	blockHash2 := cmn.RandBytes(32)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		vote := withValidator(voteProto, privValidators[0].GetAddress(), 0)
		added, err := signAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[0].GetAddress(), 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		if added {
			t.Errorf("Expected VoteSet.Add to fail, conflicting vote.")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote.")
		}
	}

	// start tracking blockHash1
	voteSet.SetPeerMaj23("peerA", BlockID{blockHash1, PartSetHeader{}})

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[0].GetAddress(), 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		if !added {
			t.Errorf("Expected VoteSet.Add to succeed, called SetPeerMaj23().")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote.")
		}
	}

	// attempt tracking blockHash2, should fail because already set for peerA.
	voteSet.SetPeerMaj23("peerA", BlockID{blockHash2, PartSetHeader{}})

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[0].GetAddress(), 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, blockHash2), voteSet)
		if added {
			t.Errorf("Expected VoteSet.Add to fail, duplicate SetPeerMaj23() from peerA")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote.")
		}
	}

	// val1 votes for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[1].GetAddress(), 1)
		added, err := signAddVote(privValidators[1], withBlockHash(vote, blockHash1), voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// check
	if voteSet.HasTwoThirdsMajority() {
		t.Errorf("We shouldn't have 2/3 majority yet")
	}
	if voteSet.HasTwoThirdsAny() {
		t.Errorf("We shouldn't have 2/3 if any votes yet")
	}

	// val2 votes for blockHash2.
	{
		vote := withValidator(voteProto, privValidators[2].GetAddress(), 2)
		added, err := signAddVote(privValidators[2], withBlockHash(vote, blockHash2), voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// check
	if voteSet.HasTwoThirdsMajority() {
		t.Errorf("We shouldn't have 2/3 majority yet")
	}
	if !voteSet.HasTwoThirdsAny() {
		t.Errorf("We should have 2/3 if any votes")
	}

	// now attempt tracking blockHash1
	voteSet.SetPeerMaj23("peerB", BlockID{blockHash1, PartSetHeader{}})

	// val2 votes for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[2].GetAddress(), 2)
		added, err := signAddVote(privValidators[2], withBlockHash(vote, blockHash1), voteSet)
		if !added {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote")
		}
	}

	// check
	if !voteSet.HasTwoThirdsMajority() {
		t.Errorf("We should have 2/3 majority for blockHash1")
	}
	blockIDMaj23, _ := voteSet.TwoThirdsMajority()
	if !bytes.Equal(blockIDMaj23.Hash, blockHash1) {
		t.Errorf("Got the wrong 2/3 majority blockhash")
	}
	if !voteSet.HasTwoThirdsAny() {
		t.Errorf("We should have 2/3 if any votes")
	}

}

func TestMakeCommit(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrecommit, 10, 1)
	blockHash, blockPartsHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Type:             VoteTypePrecommit,
		BlockID:          BlockID{blockHash, blockPartsHeader},
	}

	// 6 out of 10 voted for some block.
	for i := 0; i < 6; i++ {
		vote := withValidator(voteProto, privValidators[i].GetAddress(), i)
		signAddVote(privValidators[i], vote, voteSet)
	}

	// MakeCommit should fail.
	tst.AssertPanics(t, "Doesn't have +2/3 majority", func() { voteSet.MakeCommit() })

	// 7th voted for some other block.
	{
		vote := withValidator(voteProto, privValidators[6].GetAddress(), 6)
		vote = withBlockHash(vote, cmn.RandBytes(32))
		vote = withBlockPartsHeader(vote, PartSetHeader{123, cmn.RandBytes(32)})
		signAddVote(privValidators[6], vote, voteSet)
	}

	// The 8th voted like everyone else.
	{
		vote := withValidator(voteProto, privValidators[7].GetAddress(), 7)
		signAddVote(privValidators[7], vote, voteSet)
	}

	commit := voteSet.MakeCommit()

	// Commit should have 10 elements
	if len(commit.Precommits) != 10 {
		t.Errorf("Commit Precommits should have the same number of precommits as validators")
	}

	// Ensure that Commit precommits are ordered.
	if err := commit.ValidateBasic(); err != nil {
		t.Errorf("Error in Commit.ValidateBasic(): %v", err)
	}

}

func TestVoteSetInvalid(t *testing.T) {
	vs := (*VoteSet)(nil)
	require.Equal(t, vs.Round(), -1, "nil voteSet returns -1")
	require.Equal(t, vs.Type(), byte(0x00), "nil voteSet returns the zero value type")
	require.Equal(t, vs.Height(), 0, "nil voteSet returns the zero value height")
	require.Equal(t, vs.Size(), 0, "nil voteSet returns the zero value size")
	require.False(t, vs.HasTwoThirdsAny(), "nil voteSet cannot say if it has 2/3")
	require.False(t, vs.HasTwoThirdsMajority(), "nil voteSet cannot return 2/3 status")
	require.False(t, vs.IsCommit(), "a nil voteSet cannot tell if it is a commit")
	require.False(t, vs.HasAll(), "a nil voteSet cannot say if HasAll()")
	require.Nil(t, vs.MakeCommit(), "a nil voteSet cannot makeCommit")
	require.Contains(t, vs.StringShort(), "nil-VoteSet")
	require.Contains(t, vs.String(), "nil-VoteSet")
	require.Panics(t, func() { vs.SetPeerMaj23("foo", BlockID{}) }, "cannot set majority 2/3 on a nil voteSet")
	require.Nil(t, vs.GetByAddress([]byte("foo")), "cannot get a vote by address from a nil voteSet")
	blockID, is := vs.TwoThirdsMajority()
	require.False(t, is, "cannot be the 2/3 majority if nil")
	require.Equal(t, blockID, BlockID{}, "expecting the zero-value of BlockID")

	vs = new(VoteSet)
	require.Panics(t, func() { vs.GetByAddress([]byte("foo")) }, "non-existent address should panic")

	require.Panics(t, func() { NewVoteSet("foo", 0, 1, 0x01, nil) }, "height 0 shouldn't work")
}

func TestVoteSetEndToEnd(t *testing.T) {
	vs := (*VoteSet)(nil)
	require.Nil(t, vs.GetByIndex(0), "should return nil for any index when nil")
	require.Nil(t, vs.GetByIndex(-1), "should return nil for any index when nil")
	require.Nil(t, vs.GetByIndex(11), "should return nil for any index when nil")

	require.Panics(t, func() { vs.AddVote(nil) }, "nil voteSet should panic")

	vs, vsl, privValidators := randVoteSet(25, 2, VoteTypePrecommit, 10, 1)
	require.Equal(t, vs.Size(), 10, "expecting 10 validators as set in")
	require.Nil(t, vs.GetByIndex(0), "0: hasn't yet cast their vote")
	require.Nil(t, vs.GetByIndex(1), "1: hasn't yet cast their vote")

	// Invalid/nil/non-dereferenceable vote
	added, err := vs.AddVote(nil)
	require.False(t, added, "nil votes can't be dereferenced")
	require.NotNil(t, err, "nil votes can't be dereferenced")

	// Intentionally malicious votes
	for _, index := range []int{-1, 10000, -10} {
		added, err = vs.AddVote(&Vote{
			ValidatorIndex: index,
		})
		require.False(t, added, "%d: an invalid vote should not have been added", index)
		require.Equal(t, err, ErrVoteInvalidValidatorIndex)
	}

	// Cast some votes
	// 1. No validator address:
	v1 := &Vote{ValidatorIndex: 0}
	added, err = vs.AddVote(v1)
	require.False(t, added, "no validator address")
	require.Equal(t, err, ErrVoteInvalidValidatorAddress, "no validator address")

	// 2. Not the same Round between ValidatorSet and Vote
	val1 := vsl.Validators[0]
	v1.ValidatorAddress = val1.Address
	v1.Round = vs.Round() + 1
	added, err = vs.AddVote(v1)
	require.False(t, added, "not same round")
	require.Equal(t, err, ErrVoteUnexpectedStep, "rounds do not match")

	v1.Round = vs.Round()

	// 3. Not the same Type between ValidatorSet and Vote
	v1.Type = vs.Type() + 2
	added, err = vs.AddVote(v1)
	require.False(t, added, "not same type")
	require.Equal(t, err, ErrVoteUnexpectedStep, "not same type")

	v1.Type = vs.Type()

	// 4. Invalid height
	v1.Height = vs.Height() + 1
	added, err = vs.AddVote(v1)
	require.False(t, added, "not same height")
	require.Equal(t, err, ErrVoteUnexpectedStep, "heights don't match")

	v1.Height = vs.Height()

	// 5. Invalid signature
	randKey := crypto.GenPrivKeyEd25519()
	v1.Signature = randKey.Sign([]byte("foo-bar"))
	added, err = vs.AddVote(v1)
	require.Equal(t, err, ErrVoteInvalidSignature, "invalid signature")

	// 6. Valid signature
	priv1 := privValidators[0]
	require.Nil(t, priv1.SignVote(vs.ChainID(), v1))
	added, err = vs.AddVote(v1)

	nonNilCount := func(vs *VoteSet) int {
		count := 0
		for _, v := range vs.votes {
			if v != nil {
				count += 1
			}
		}
		return count
	}
	require.True(t, added, "expected the vote to have been cast")
	require.Nil(t, err, "no errors in casting the vote")
	require.Equal(t, nonNilCount(vs), 1, "only one vote cast so far")

	// 7. Duplicate vote, right signature
	added, err = vs.AddVote(v1)
	require.False(t, added, "duplicate vote, already seen")
	require.Nil(t, err, "duplicate but has the right signature")
	require.Equal(t, nonNilCount(vs), 1, "only one vote despite a duplicate attempt")

	origIndex := v1.ValidatorIndex
	origBlockKey := v1.BlockID.Key()

	// 8. Mutated vote shouldn't affect the already cast vote
	// Firstly check that we only have one vote
	require.Equal(t, nonNilCount(vs), 1, "only one vote cast")
	v1.Round = vs.Round() + 10
	val2 := vsl.Validators[1]
	v1.ValidatorAddress = val2.Address

	castVote, ok := vs.getVote(origIndex, origBlockKey)
	require.True(t, ok, "expecting successful retrieval of the previously cast vote")
	require.NotNil(t, castVote, "expecting a non-nil vote")
	require.NotEqual(t, v1, castVote, "despite v1 being cast already, a Copy() must have been made, hence immutable")

	// 9. Sneaky change of validator address
	v1 = castVote.Copy()
	v1.ValidatorIndex = 1
	added, err = vs.AddVote(v1)
	require.False(t, added, "different validators at different addresses")
	require.Equal(t, err, ErrVoteInvalidValidatorAddress, "different validators at different addresses")

	// 10. Change of the validator index whimsically, but they aren't a validator
	v1 = castVote.Copy()
	v1.ValidatorIndex = 10000
	added, err = vs.AddVote(v1)
	require.False(t, added, "different validators at different addresses")
	require.Equal(t, err, ErrVoteInvalidValidatorIndex, "different validators at different addresses")

	// 11. Change of signature
	v1 = castVote.Copy()
	v1.Signature = randKey.Sign([]byte("foo-bar"))
	added, err = vs.AddVote(v1)
	require.False(t, added, "different/unknown signature")
	require.Equal(t, err, ErrVoteInvalidSignature, "different/unknown/undeterministic signature")
}
