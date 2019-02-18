package types

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestValidatorSetBasic(t *testing.T) {
	// empty or nil validator lists are allowed,
	// but attempting to IncrementProposerPriority on them will panic.
	vset := NewValidatorSet([]*Validator{})
	assert.Panics(t, func() { vset.IncrementProposerPriority(1) })

	vset = NewValidatorSet(nil)
	assert.Panics(t, func() { vset.IncrementProposerPriority(1) })

	assert.EqualValues(t, vset, vset.Copy())
	assert.False(t, vset.HasAddress([]byte("some val")))
	idx, val := vset.GetByAddress([]byte("some val"))
	assert.Equal(t, -1, idx)
	assert.Nil(t, val)
	addr, val := vset.GetByIndex(-100)
	assert.Nil(t, addr)
	assert.Nil(t, val)
	addr, val = vset.GetByIndex(0)
	assert.Nil(t, addr)
	assert.Nil(t, val)
	addr, val = vset.GetByIndex(100)
	assert.Nil(t, addr)
	assert.Nil(t, val)
	assert.Zero(t, vset.Size())
	assert.Equal(t, int64(0), vset.TotalVotingPower())
	assert.Nil(t, vset.GetProposer())
	assert.Nil(t, vset.Hash())

	// add
	val = randValidator_(vset.TotalVotingPower())
	assert.NoError(t, vset.UpdateWithChangeSet([]*Validator{val}))

	assert.True(t, vset.HasAddress(val.Address))
	idx, _ = vset.GetByAddress(val.Address)
	assert.Equal(t, 0, idx)
	addr, _ = vset.GetByIndex(0)
	assert.Equal(t, []byte(val.Address), addr)
	assert.Equal(t, 1, vset.Size())
	assert.Equal(t, val.VotingPower, vset.TotalVotingPower())
	assert.NotNil(t, vset.Hash())
	assert.NotPanics(t, func() { vset.IncrementProposerPriority(1) })
	assert.Equal(t, val.Address, vset.GetProposer().Address)

	// update
	val = randValidator_(vset.TotalVotingPower())
	assert.NoError(t, vset.UpdateWithChangeSet([]*Validator{val}))
	_, val = vset.GetByAddress(val.Address)
	val.VotingPower += 100
	proposerPriority := val.ProposerPriority

	val.ProposerPriority = 0
	assert.NoError(t, vset.UpdateWithChangeSet([]*Validator{val}))
	_, val = vset.GetByAddress(val.Address)
	assert.Equal(t, proposerPriority, val.ProposerPriority)

	// remove
	val2, removed := vset.Remove(randValidator_(vset.TotalVotingPower()).Address)
	assert.Nil(t, val2)
	assert.False(t, removed)
	val2, removed = vset.Remove(val.Address)
	assert.Equal(t, val.Address, val2.Address)
	assert.True(t, removed)
}

func TestCopy(t *testing.T) {
	vset := randValidatorSet(10)
	vsetHash := vset.Hash()
	if len(vsetHash) == 0 {
		t.Fatalf("ValidatorSet had unexpected zero hash")
	}

	vsetCopy := vset.Copy()
	vsetCopyHash := vsetCopy.Hash()

	if !bytes.Equal(vsetHash, vsetCopyHash) {
		t.Fatalf("ValidatorSet copy had wrong hash. Orig: %X, Copy: %X", vsetHash, vsetCopyHash)
	}
}

// Test that IncrementProposerPriority requires positive times.
func TestIncrementProposerPriorityPositiveTimes(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		newValidator([]byte("foo"), 1000),
		newValidator([]byte("bar"), 300),
		newValidator([]byte("baz"), 330),
	})

	assert.Panics(t, func() { vset.IncrementProposerPriority(-1) })
	assert.Panics(t, func() { vset.IncrementProposerPriority(0) })
	vset.IncrementProposerPriority(1)
}

func BenchmarkValidatorSetCopy(b *testing.B) {
	b.StopTimer()
	vset := NewValidatorSet([]*Validator{})
	for i := 0; i < 1000; i++ {
		privKey := ed25519.GenPrivKey()
		pubKey := privKey.PubKey()
		val := NewValidator(pubKey, 10)
		err := vset.UpdateWithChangeSet([]*Validator{val})
		if err != nil {
			panic("Failed to add validator")
		}
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		vset.Copy()
	}
}

//-------------------------------------------------------------------

func TestProposerSelection1(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		newValidator([]byte("foo"), 1000),
		newValidator([]byte("bar"), 300),
		newValidator([]byte("baz"), 330),
	})
	var proposers []string
	for i := 0; i < 99; i++ {
		val := vset.GetProposer()
		proposers = append(proposers, string(val.Address))
		vset.IncrementProposerPriority(1)
	}
	expected := `foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo foo baz bar foo foo foo baz foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo`
	if expected != strings.Join(proposers, " ") {
		t.Errorf("Expected sequence of proposers was\n%v\nbut got \n%v", expected, strings.Join(proposers, " "))
	}
}

func TestProposerSelection2(t *testing.T) {
	addr0 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	addr1 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	addr2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}

	// when all voting power is same, we go in order of addresses
	val0, val1, val2 := newValidator(addr0, 100), newValidator(addr1, 100), newValidator(addr2, 100)
	valList := []*Validator{val0, val1, val2}
	vals := NewValidatorSet(valList)
	for i := 0; i < len(valList)*5; i++ {
		ii := (i) % len(valList)
		prop := vals.GetProposer()
		if !bytes.Equal(prop.Address, valList[ii].Address) {
			t.Fatalf("(%d): Expected %X. Got %X", i, valList[ii].Address, prop.Address)
		}
		vals.IncrementProposerPriority(1)
	}

	// One validator has more than the others, but not enough to propose twice in a row
	*val2 = *newValidator(addr2, 400)
	vals = NewValidatorSet(valList)
	// vals.IncrementProposerPriority(1)
	prop := vals.GetProposer()
	if !bytes.Equal(prop.Address, addr2) {
		t.Fatalf("Expected address with highest voting power to be first proposer. Got %X", prop.Address)
	}
	vals.IncrementProposerPriority(1)
	prop = vals.GetProposer()
	if !bytes.Equal(prop.Address, addr0) {
		t.Fatalf("Expected smallest address to be validator. Got %X", prop.Address)
	}

	// One validator has more than the others, and enough to be proposer twice in a row
	*val2 = *newValidator(addr2, 401)
	vals = NewValidatorSet(valList)
	prop = vals.GetProposer()
	if !bytes.Equal(prop.Address, addr2) {
		t.Fatalf("Expected address with highest voting power to be first proposer. Got %X", prop.Address)
	}
	vals.IncrementProposerPriority(1)
	prop = vals.GetProposer()
	if !bytes.Equal(prop.Address, addr2) {
		t.Fatalf("Expected address with highest voting power to be second proposer. Got %X", prop.Address)
	}
	vals.IncrementProposerPriority(1)
	prop = vals.GetProposer()
	if !bytes.Equal(prop.Address, addr0) {
		t.Fatalf("Expected smallest address to be validator. Got %X", prop.Address)
	}

	// each validator should be the proposer a proportional number of times
	val0, val1, val2 = newValidator(addr0, 4), newValidator(addr1, 5), newValidator(addr2, 3)
	valList = []*Validator{val0, val1, val2}
	propCount := make([]int, 3)
	vals = NewValidatorSet(valList)
	N := 1
	for i := 0; i < 120*N; i++ {
		prop := vals.GetProposer()
		ii := prop.Address[19]
		propCount[ii]++
		vals.IncrementProposerPriority(1)
	}

	if propCount[0] != 40*N {
		t.Fatalf("Expected prop count for validator with 4/12 of voting power to be %d/%d. Got %d/%d", 40*N, 120*N, propCount[0], 120*N)
	}
	if propCount[1] != 50*N {
		t.Fatalf("Expected prop count for validator with 5/12 of voting power to be %d/%d. Got %d/%d", 50*N, 120*N, propCount[1], 120*N)
	}
	if propCount[2] != 30*N {
		t.Fatalf("Expected prop count for validator with 3/12 of voting power to be %d/%d. Got %d/%d", 30*N, 120*N, propCount[2], 120*N)
	}
}

func TestProposerSelection3(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		newValidator([]byte("a"), 1),
		newValidator([]byte("b"), 1),
		newValidator([]byte("c"), 1),
		newValidator([]byte("d"), 1),
	})

	proposerOrder := make([]*Validator, 4)
	for i := 0; i < 4; i++ {
		proposerOrder[i] = vset.GetProposer()
		vset.IncrementProposerPriority(1)
	}

	// i for the loop
	// j for the times
	// we should go in order for ever, despite some IncrementProposerPriority with times > 1
	var i, j int
	for ; i < 10000; i++ {
		got := vset.GetProposer().Address
		expected := proposerOrder[j%4].Address
		if !bytes.Equal(got, expected) {
			t.Fatalf(fmt.Sprintf("vset.Proposer (%X) does not match expected proposer (%X) for (%d, %d)", got, expected, i, j))
		}

		// serialize, deserialize, check proposer
		b := vset.toBytes()
		vset.fromBytes(b)

		computed := vset.GetProposer() // findGetProposer()
		if i != 0 {
			if !bytes.Equal(got, computed.Address) {
				t.Fatalf(fmt.Sprintf("vset.Proposer (%X) does not match computed proposer (%X) for (%d, %d)", got, computed.Address, i, j))
			}
		}

		// times is usually 1
		times := 1
		mod := (cmn.RandInt() % 5) + 1
		if cmn.RandInt()%mod > 0 {
			// sometimes its up to 5
			times = (cmn.RandInt() % 4) + 1
		}
		vset.IncrementProposerPriority(times)

		j += times
	}
}

func newValidator(address []byte, power int64) *Validator {
	return &Validator{Address: address, VotingPower: power}
}

func randPubKey() crypto.PubKey {
	var pubKey [32]byte
	copy(pubKey[:], cmn.RandBytes(32))
	return ed25519.PubKeyEd25519(pubKey)
}

func randValidator_(totalVotingPower int64) *Validator {
	// this modulo limits the ProposerPriority/VotingPower to stay in the
	// bounds of MaxTotalVotingPower minus the already existing voting power:
	val := NewValidator(randPubKey(), int64(cmn.RandUint64()%uint64((MaxTotalVotingPower-totalVotingPower))))
	val.ProposerPriority = cmn.RandInt64() % (MaxTotalVotingPower - totalVotingPower)
	return val
}

func randValidatorSet(numValidators int) *ValidatorSet {
	validators := make([]*Validator, numValidators)
	totalVotingPower := int64(0)
	for i := 0; i < numValidators; i++ {
		validators[i] = randValidator_(totalVotingPower)
		totalVotingPower += validators[i].VotingPower
	}
	return NewValidatorSet(validators)
}

func (valSet *ValidatorSet) toBytes() []byte {
	bz, err := cdc.MarshalBinaryLengthPrefixed(valSet)
	if err != nil {
		panic(err)
	}
	return bz
}

func (valSet *ValidatorSet) fromBytes(b []byte) {
	err := cdc.UnmarshalBinaryLengthPrefixed(b, &valSet)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		panic(err)
	}
}

//-------------------------------------------------------------------

func TestValidatorSetTotalVotingPowerPanicsOnOverflow(t *testing.T) {
	// NewValidatorSet calls IncrementProposerPriority which calls TotalVotingPower()
	// which should panic on overflows:
	shouldPanic := func() {
		NewValidatorSet([]*Validator{
			{Address: []byte("a"), VotingPower: math.MaxInt64, ProposerPriority: 0},
			{Address: []byte("b"), VotingPower: math.MaxInt64, ProposerPriority: 0},
			{Address: []byte("c"), VotingPower: math.MaxInt64, ProposerPriority: 0},
		})
	}

	assert.Panics(t, shouldPanic)
}

func TestAvgProposerPriority(t *testing.T) {
	// Create Validator set without calling IncrementProposerPriority:
	tcs := []struct {
		vs   ValidatorSet
		want int64
	}{
		0: {ValidatorSet{Validators: []*Validator{{ProposerPriority: 0}, {ProposerPriority: 0}, {ProposerPriority: 0}}}, 0},
		1: {ValidatorSet{Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: 0}, {ProposerPriority: 0}}}, math.MaxInt64 / 3},
		2: {ValidatorSet{Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: 0}}}, math.MaxInt64 / 2},
		3: {ValidatorSet{Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: math.MaxInt64}}}, math.MaxInt64},
		4: {ValidatorSet{Validators: []*Validator{{ProposerPriority: math.MinInt64}, {ProposerPriority: math.MinInt64}}}, math.MinInt64},
	}
	for i, tc := range tcs {
		got := tc.vs.computeAvgProposerPriority()
		assert.Equal(t, tc.want, got, "test case: %v", i)
	}
}

func TestAveragingInIncrementProposerPriority(t *testing.T) {
	// Test that the averaging works as expected inside of IncrementProposerPriority.
	// Each validator comes with zero voting power which simplifies reasoning about
	// the expected ProposerPriority.
	tcs := []struct {
		vs    ValidatorSet
		times int
		avg   int64
	}{
		0: {ValidatorSet{
			Validators: []*Validator{
				{Address: []byte("a"), ProposerPriority: 1},
				{Address: []byte("b"), ProposerPriority: 2},
				{Address: []byte("c"), ProposerPriority: 3}}},
			1, 2},
		1: {ValidatorSet{
			Validators: []*Validator{
				{Address: []byte("a"), ProposerPriority: 10},
				{Address: []byte("b"), ProposerPriority: -10},
				{Address: []byte("c"), ProposerPriority: 1}}},
			// this should average twice but the average should be 0 after the first iteration
			// (voting power is 0 -> no changes)
			11, 1 / 3},
		2: {ValidatorSet{
			Validators: []*Validator{
				{Address: []byte("a"), ProposerPriority: 100},
				{Address: []byte("b"), ProposerPriority: -10},
				{Address: []byte("c"), ProposerPriority: 1}}},
			1, 91 / 3},
	}
	for i, tc := range tcs {
		// work on copy to have the old ProposerPriorities:
		newVset := tc.vs.CopyIncrementProposerPriority(tc.times)
		for _, val := range tc.vs.Validators {
			_, updatedVal := newVset.GetByAddress(val.Address)
			assert.Equal(t, updatedVal.ProposerPriority, val.ProposerPriority-tc.avg, "test case: %v", i)
		}
	}
}

func TestAveragingInIncrementProposerPriorityWithVotingPower(t *testing.T) {
	// Other than TestAveragingInIncrementProposerPriority this is a more complete test showing
	// how each ProposerPriority changes in relation to the validator's voting power respectively.
	// average is zero in each round:
	vp0 := int64(10)
	vp1 := int64(1)
	vp2 := int64(1)
	total := vp0 + vp1 + vp2
	avg := (vp0 + vp1 + vp2 - total) / 3
	vals := ValidatorSet{Validators: []*Validator{
		{Address: []byte{0}, ProposerPriority: 0, VotingPower: vp0},
		{Address: []byte{1}, ProposerPriority: 0, VotingPower: vp1},
		{Address: []byte{2}, ProposerPriority: 0, VotingPower: vp2}}}
	tcs := []struct {
		vals                  *ValidatorSet
		wantProposerPrioritys []int64
		times                 int
		wantProposer          *Validator
	}{

		0: {
			vals.Copy(),
			[]int64{
				// Acumm+VotingPower-Avg:
				0 + vp0 - total - avg, // mostest will be subtracted by total voting power (12)
				0 + vp1,
				0 + vp2},
			1,
			vals.Validators[0]},
		1: {
			vals.Copy(),
			[]int64{
				(0 + vp0 - total) + vp0 - total - avg, // this will be mostest on 2nd iter, too
				(0 + vp1) + vp1,
				(0 + vp2) + vp2},
			2,
			vals.Validators[0]}, // increment twice -> expect average to be subtracted twice
		2: {
			vals.Copy(),
			[]int64{
				0 + 3*(vp0-total) - avg, // still mostest
				0 + 3*vp1,
				0 + 3*vp2},
			3,
			vals.Validators[0]},
		3: {
			vals.Copy(),
			[]int64{
				0 + 4*(vp0-total), // still mostest
				0 + 4*vp1,
				0 + 4*vp2},
			4,
			vals.Validators[0]},
		4: {
			vals.Copy(),
			[]int64{
				0 + 4*(vp0-total) + vp0, // 4 iters was mostest
				0 + 5*vp1 - total,       // now this val is mostest for the 1st time (hence -12==totalVotingPower)
				0 + 5*vp2},
			5,
			vals.Validators[1]},
		5: {
			vals.Copy(),
			[]int64{
				0 + 6*vp0 - 5*total, // mostest again
				0 + 6*vp1 - total,   // mostest once up to here
				0 + 6*vp2},
			6,
			vals.Validators[0]},
		6: {
			vals.Copy(),
			[]int64{
				0 + 7*vp0 - 6*total, // in 7 iters this val is mostest 6 times
				0 + 7*vp1 - total,   // in 7 iters this val is mostest 1 time
				0 + 7*vp2},
			7,
			vals.Validators[0]},
		7: {
			vals.Copy(),
			[]int64{
				0 + 8*vp0 - 7*total, // mostest again
				0 + 8*vp1 - total,
				0 + 8*vp2},
			8,
			vals.Validators[0]},
		8: {
			vals.Copy(),
			[]int64{
				0 + 9*vp0 - 7*total,
				0 + 9*vp1 - total,
				0 + 9*vp2 - total}, // mostest
			9,
			vals.Validators[2]},
		9: {
			vals.Copy(),
			[]int64{
				0 + 10*vp0 - 8*total, // after 10 iters this is mostest again
				0 + 10*vp1 - total,   // after 6 iters this val is "mostest" once and not in between
				0 + 10*vp2 - total},  // in between 10 iters this val is "mostest" once
			10,
			vals.Validators[0]},
		10: {
			vals.Copy(),
			[]int64{
				0 + 11*vp0 - 9*total,
				0 + 11*vp1 - total,  // after 6 iters this val is "mostest" once and not in between
				0 + 11*vp2 - total}, // after 10 iters this val is "mostest" once
			11,
			vals.Validators[0]},
	}
	for i, tc := range tcs {
		tc.vals.IncrementProposerPriority(tc.times)

		assert.Equal(t, tc.wantProposer.Address, tc.vals.GetProposer().Address,
			"test case: %v",
			i)

		for valIdx, val := range tc.vals.Validators {
			assert.Equal(t,
				tc.wantProposerPrioritys[valIdx],
				val.ProposerPriority,
				"test case: %v, validator: %v",
				i,
				valIdx)
		}
	}
}

func TestSafeAdd(t *testing.T) {
	f := func(a, b int64) bool {
		c, overflow := safeAdd(a, b)
		return overflow || (!overflow && c == a+b)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSafeAddClip(t *testing.T) {
	assert.EqualValues(t, math.MaxInt64, safeAddClip(math.MaxInt64, 10))
	assert.EqualValues(t, math.MaxInt64, safeAddClip(math.MaxInt64, math.MaxInt64))
	assert.EqualValues(t, math.MinInt64, safeAddClip(math.MinInt64, -10))
}

func TestSafeSubClip(t *testing.T) {
	assert.EqualValues(t, math.MinInt64, safeSubClip(math.MinInt64, 10))
	assert.EqualValues(t, 0, safeSubClip(math.MinInt64, math.MinInt64))
	assert.EqualValues(t, math.MinInt64, safeSubClip(math.MinInt64, math.MaxInt64))
	assert.EqualValues(t, math.MaxInt64, safeSubClip(math.MaxInt64, -10))
}

//-------------------------------------------------------------------

func TestValidatorSetVerifyCommit(t *testing.T) {
	privKey := ed25519.GenPrivKey()
	pubKey := privKey.PubKey()
	v1 := NewValidator(pubKey, 1000)
	vset := NewValidatorSet([]*Validator{v1})

	chainID := "mychainID"
	blockID := BlockID{Hash: []byte("hello")}
	height := int64(5)
	vote := &Vote{
		ValidatorAddress: v1.Address,
		ValidatorIndex:   0,
		Height:           height,
		Round:            0,
		Timestamp:        tmtime.Now(),
		Type:             PrecommitType,
		BlockID:          blockID,
	}
	sig, err := privKey.Sign(vote.SignBytes(chainID))
	assert.NoError(t, err)
	vote.Signature = sig
	commit := NewCommit(blockID, []*CommitSig{vote.CommitSig()})

	badChainID := "notmychainID"
	badBlockID := BlockID{Hash: []byte("goodbye")}
	badHeight := height + 1
	badCommit := NewCommit(blockID, []*CommitSig{nil})

	// test some error cases
	// TODO: test more cases!
	cases := []struct {
		chainID string
		blockID BlockID
		height  int64
		commit  *Commit
	}{
		{badChainID, blockID, height, commit},
		{chainID, badBlockID, height, commit},
		{chainID, blockID, badHeight, commit},
		{chainID, blockID, height, badCommit},
	}

	for i, c := range cases {
		err := vset.VerifyCommit(c.chainID, c.blockID, c.height, c.commit)
		assert.NotNil(t, err, i)
	}

	// test a good one
	err = vset.VerifyCommit(chainID, blockID, height, commit)
	assert.Nil(t, err)
}

func TestEmptySet(t *testing.T) {

	var valList []*Validator
	valSet := NewValidatorSet(valList)
	assert.Panics(t, func() { valSet.IncrementProposerPriority(1) })
	assert.Panics(t, func() { valSet.RescalePriorities(100) })
	assert.Panics(t, func() { valSet.shiftByAvgProposerPriority() })
	assert.Panics(t, func() { assert.Zero(t, computeMaxMinPriorityDiff(valSet)) })
	valSet.GetProposer()

	// Add to empty set
	v1 := newValidator([]byte("v1"), 100)
	v2 := newValidator([]byte("v2"), 100)
	valList = []*Validator{v1, v2}
	assert.NoError(t, valSet.UpdateWithChangeSet(valList))
	verifyValidatorSet(t, valSet)

	// Delete all validators from set
	v1 = newValidator([]byte("v1"), 0)
	v2 = newValidator([]byte("v2"), 0)
	delList := []*Validator{v1, v2}
	assert.Error(t, valSet.UpdateWithChangeSet(delList))

	// Attempt delete from empty set
	assert.Error(t, valSet.UpdateWithChangeSet(delList))

}

func TestUpdatesForNewValidatorSet(t *testing.T) {

	v1 := newValidator([]byte("v1"), 100)
	v2 := newValidator([]byte("v2"), 100)
	valList := []*Validator{v1, v2}
	valSet := NewValidatorSet(valList)
	verifyValidatorSet(t, valSet)

	// Verify duplicates are caught in NewValidatorSet() and it panics
	v111 := newValidator([]byte("v1"), 100)
	v112 := newValidator([]byte("v1"), 123)
	v113 := newValidator([]byte("v1"), 234)
	valList = []*Validator{v111, v112, v113}
	assert.Panics(t, func() { NewValidatorSet(valList) })

	// Verify set including validator with voting power 0 cannot be created
	v1 = newValidator([]byte("v1"), 0)
	v2 = newValidator([]byte("v2"), 22)
	v3 := newValidator([]byte("v3"), 33)
	valList = []*Validator{v1, v2, v3}
	assert.Panics(t, func() { NewValidatorSet(valList) })

	// Verify set including validator with negative voting power cannot be created
	v1 = newValidator([]byte("v1"), 10)
	v2 = newValidator([]byte("v2"), -20)
	v3 = newValidator([]byte("v3"), 30)
	valList = []*Validator{v1, v2, v3}
	assert.Panics(t, func() { NewValidatorSet(valList) })

}

type testVal struct {
	name  string
	power int64
}

func testValSet(nVals int, power int64) []testVal {
	vals := make([]testVal, nVals)
	for i := 0; i < nVals; i++ {
		vals[i] = testVal{fmt.Sprintf("v%d", i+1), power}
	}
	return vals
}

type valSetErrTestCase struct {
	startVals  []testVal
	updateVals []testVal
}

func executeValSetErrTestCase(t *testing.T, idx int, tt valSetErrTestCase) {
	// create a new set and apply updates, keeping copies for the checks
	valSet := createNewValidatorSet(tt.startVals)
	valSetCopy := valSet.Copy()
	valList := createNewValidatorList(tt.updateVals)
	valListCopy := validatorListCopy(valList)
	err := valSet.UpdateWithChangeSet(valList)

	// for errors check the validator set has not been changed
	assert.Error(t, err, "test %d", idx)
	assert.Equal(t, valSet, valSetCopy, "test %v", idx)

	// check the parameter list has not changed
	assert.Equal(t, valList, valListCopy, "test %v", idx)
}

func TestValSetUpdatesDuplicateEntries(t *testing.T) {
	testCases := []valSetErrTestCase{
		// Duplicate entries in changes
		{ // first entry is duplicated change
			testValSet(2, 10),
			[]testVal{{"v1", 11}, {"v1", 22}},
		},
		{ // second entry is duplicated change
			testValSet(2, 10),
			[]testVal{{"v2", 11}, {"v2", 22}},
		},
		{ // change duplicates are separated by a valid change
			testValSet(2, 10),
			[]testVal{{"v1", 11}, {"v2", 22}, {"v1", 12}},
		},
		{ // change duplicates are separated by a valid change
			testValSet(3, 10),
			[]testVal{{"v1", 11}, {"v3", 22}, {"v1", 12}},
		},

		// Duplicate entries in remove
		{ // first entry is duplicated remove
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v1", 0}},
		},
		{ // second entry is duplicated remove
			testValSet(2, 10),
			[]testVal{{"v2", 0}, {"v2", 0}},
		},
		{ // remove duplicates are separated by a valid remove
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v2", 0}, {"v1", 0}},
		},
		{ // remove duplicates are separated by a valid remove
			testValSet(3, 10),
			[]testVal{{"v1", 0}, {"v3", 0}, {"v1", 0}},
		},

		{ // remove and update same val
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v2", 20}, {"v1", 30}},
		},
		{ // duplicate entries in removes + changes
			testValSet(2, 10),
			[]testVal{{"v1", 0}, {"v2", 20}, {"v2", 30}, {"v1", 0}},
		},
		{ // duplicate entries in removes + changes
			testValSet(3, 10),
			[]testVal{{"v1", 0}, {"v3", 5}, {"v2", 20}, {"v2", 30}, {"v1", 0}},
		},
	}

	for i, tt := range testCases {
		executeValSetErrTestCase(t, i, tt)
	}
}

func TestValSetUpdatesOverflows(t *testing.T) {
	maxVP := MaxTotalVotingPower
	testCases := []valSetErrTestCase{
		{ // single update leading to overflow
			testValSet(2, 10),
			[]testVal{{"v1", math.MaxInt64}},
		},
		{ // single update leading to overflow
			testValSet(2, 10),
			[]testVal{{"v2", math.MaxInt64}},
		},
		{ // add validator leading to exceed Max
			testValSet(1, maxVP-1),
			[]testVal{{"v2", 5}},
		},
		{ // add validator leading to exceed Max
			testValSet(2, maxVP/3),
			[]testVal{{"v3", maxVP / 2}},
		},
	}

	for i, tt := range testCases {
		executeValSetErrTestCase(t, i, tt)
	}
}

func TestValSetUpdatesOtherErrors(t *testing.T) {
	testCases := []valSetErrTestCase{
		{ // update with negative voting power
			testValSet(2, 10),
			[]testVal{{"v1", -123}},
		},
		{ // update with negative voting power
			testValSet(2, 10),
			[]testVal{{"v2", -123}},
		},
		{ // remove non-existing validator
			testValSet(2, 10),
			[]testVal{{"v3", 0}},
		},
		{ // delete all validators
			[]testVal{{"v1", 10}, {"v2", 20}, {"v3", 30}},
			[]testVal{{"v1", 0}, {"v2", 0}, {"v3", 0}},
		},
	}

	for i, tt := range testCases {
		executeValSetErrTestCase(t, i, tt)
	}
}

func TestValSetUpdatesBasicTestsExecute(t *testing.T) {
	valSetUpdatesBasicTests := []struct {
		startVals    []testVal
		updateVals   []testVal
		expectedVals []testVal
	}{
		{ // no changes
			testValSet(2, 10),
			[]testVal{},
			testValSet(2, 10),
		},
		{ // voting power changes
			testValSet(2, 10),
			[]testVal{{"v1", 11}, {"v2", 22}},
			[]testVal{{"v1", 11}, {"v2", 22}},
		},
		{ // add new validators
			[]testVal{{"v1", 10}, {"v2", 20}},
			[]testVal{{"v3", 30}, {"v4", 40}},
			[]testVal{{"v1", 10}, {"v2", 20}, {"v3", 30}, {"v4", 40}},
		},
		{ // add new validator to middle
			[]testVal{{"v1", 10}, {"v3", 20}},
			[]testVal{{"v2", 30}},
			[]testVal{{"v1", 10}, {"v2", 30}, {"v3", 20}},
		},
		{ // add new validator to beginning
			[]testVal{{"v2", 10}, {"v3", 20}},
			[]testVal{{"v1", 30}},
			[]testVal{{"v1", 30}, {"v2", 10}, {"v3", 20}},
		},
		{ // delete validators
			[]testVal{{"v1", 10}, {"v2", 20}, {"v3", 30}},
			[]testVal{{"v2", 0}},
			[]testVal{{"v1", 10}, {"v3", 30}},
		},
	}

	for i, tt := range valSetUpdatesBasicTests {
		// create a new set and apply updates, keeping copies for the checks
		valSet := createNewValidatorSet(tt.startVals)
		valList := createNewValidatorList(tt.updateVals)
		valListCopy := validatorListCopy(valList)
		err := valSet.UpdateWithChangeSet(valList)
		assert.NoError(t, err, "test %d", i)

		// check the parameter list has not changed
		assert.Equal(t, valList, valListCopy, "test %v", i)

		// check the final validator list is as expected and the set is properly scaled and centered.
		assert.Equal(t, getValidatorResults(valSet.Validators), tt.expectedVals, "test %v", i)
		verifyValidatorSet(t, valSet)
	}
}

func getValidatorResults(valList []*Validator) []testVal {
	testList := make([]testVal, len(valList))
	for i, val := range valList {
		testList[i].name = string(val.Address)
		testList[i].power = val.VotingPower
	}
	return testList
}

// Test that different permutations of an update give the same result.
func TestValSetUpdatesOrderTestsExecute(t *testing.T) {
	// startVals - initial validators to create the set with
	// updateVals - a sequence of updates to be applied to the set.
	// updateVals is shuffled a number of times during testing to check for same resulting validator set.
	valSetUpdatesOrderTests := []struct {
		startVals  []testVal
		updateVals []testVal
	}{
		0: { // order of changes should not matter, the final validator sets should be the same
			[]testVal{{"v1", 10}, {"v2", 10}, {"v3", 30}, {"v4", 40}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}, {"v4", 44}}},

		1: { // order of additions should not matter
			[]testVal{{"v1", 10}, {"v2", 20}},
			[]testVal{{"v3", 30}, {"v4", 40}, {"v5", 50}, {"v6", 60}}},

		2: { // order of removals should not matter
			[]testVal{{"v1", 10}, {"v2", 20}, {"v3", 30}, {"v4", 40}},
			[]testVal{{"v1", 0}, {"v3", 0}, {"v4", 0}}},

		3: { // order of mixed operations should not matter
			[]testVal{{"v1", 10}, {"v2", 20}, {"v3", 30}, {"v4", 40}},
			[]testVal{{"v1", 0}, {"v3", 0}, {"v2", 22}, {"v5", 50}, {"v4", 44}}},
	}

	for i, tt := range valSetUpdatesOrderTests {
		// create a new set and apply updates
		valSet := createNewValidatorSet(tt.startVals)
		valSetCopy := valSet.Copy()
		valList := createNewValidatorList(tt.updateVals)
		assert.NoError(t, valSetCopy.UpdateWithChangeSet(valList))

		// save the result as expected for next updates
		valSetExp := valSetCopy.Copy()

		// perform at most 20 permutations on the updates and call UpdateWithChangeSet()
		n := len(tt.updateVals)
		maxNumPerms := cmn.MinInt(20, n*n)
		for j := 0; j < maxNumPerms; j++ {
			// create a copy of original set and apply a random permutation of updates
			valSetCopy := valSet.Copy()
			valList := createNewValidatorList(permutation(tt.updateVals))

			// check there was no error and the set is properly scaled and centered.
			assert.NoError(t, valSetCopy.UpdateWithChangeSet(valList),
				"test %v failed for permutation %v", i, valList)
			verifyValidatorSet(t, valSetCopy)

			// verify the resulting test is same as the expected
			assert.Equal(t, valSetCopy, valSetExp,
				"test %v failed for permutation %v", i, valList)
		}
	}
}

// This tests the private function validator_set.go:applyUpdates() function, used only for additions and changes.
// Should perform a proper merge of updatedVals and startVals
func TestValSetApplyUpdatesTestsExecute(t *testing.T) {
	valSetUpdatesBasicTests := []struct {
		startVals    []testVal
		updateVals   []testVal
		expectedVals []testVal
	}{
		// additions
		0: { // prepend
			[]testVal{{"v4", 44}, {"v5", 55}},
			[]testVal{{"v1", 11}},
			[]testVal{{"v1", 11}, {"v4", 44}, {"v5", 55}}},
		1: { // append
			[]testVal{{"v4", 44}, {"v5", 55}},
			[]testVal{{"v6", 66}},
			[]testVal{{"v4", 44}, {"v5", 55}, {"v6", 66}}},
		2: { // insert
			[]testVal{{"v4", 44}, {"v6", 66}},
			[]testVal{{"v5", 55}},
			[]testVal{{"v4", 44}, {"v5", 55}, {"v6", 66}}},
		3: { // insert multi
			[]testVal{{"v4", 44}, {"v6", 66}, {"v9", 99}},
			[]testVal{{"v5", 55}, {"v7", 77}, {"v8", 88}},
			[]testVal{{"v4", 44}, {"v5", 55}, {"v6", 66}, {"v7", 77}, {"v8", 88}, {"v9", 99}}},
		// changes
		4: { // head
			[]testVal{{"v1", 111}, {"v2", 22}},
			[]testVal{{"v1", 11}},
			[]testVal{{"v1", 11}, {"v2", 22}}},
		5: { // tail
			[]testVal{{"v1", 11}, {"v2", 222}},
			[]testVal{{"v2", 22}},
			[]testVal{{"v1", 11}, {"v2", 22}}},
		6: { // middle
			[]testVal{{"v1", 11}, {"v2", 222}, {"v3", 33}},
			[]testVal{{"v2", 22}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}}},
		7: { // multi
			[]testVal{{"v1", 111}, {"v2", 222}, {"v3", 333}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}}},
		// additions and changes
		8: {
			[]testVal{{"v1", 111}, {"v2", 22}},
			[]testVal{{"v1", 11}, {"v3", 33}, {"v4", 44}},
			[]testVal{{"v1", 11}, {"v2", 22}, {"v3", 33}, {"v4", 44}}},
	}

	for i, tt := range valSetUpdatesBasicTests {
		// create a new validator set with the start values
		valSet := createNewValidatorSet(tt.startVals)

		// applyUpdates() with the update values
		valList := createNewValidatorList(tt.updateVals)
		valSet.applyUpdates(valList)

		// check the new list of validators for proper merge
		assert.Equal(t, getValidatorResults(valSet.Validators), tt.expectedVals, "test %v", i)
		verifyValidatorSet(t, valSet)
	}
}

func permutation(valList []testVal) []testVal {
	if len(valList) == 0 {
		return nil
	}
	permList := make([]testVal, len(valList))
	perm := rand.Perm(len(valList))
	for i, v := range perm {
		permList[v] = valList[i]
	}
	return permList
}

func createNewValidatorList(testValList []testVal) []*Validator {
	valList := make([]*Validator, 0, len(testValList))
	for _, val := range testValList {
		valList = append(valList, newValidator([]byte(val.name), val.power))
	}
	return valList
}

func createNewValidatorSet(testValList []testVal) *ValidatorSet {
	valList := createNewValidatorList(testValList)
	valSet := NewValidatorSet(valList)
	return valSet
}

func verifyValidatorSet(t *testing.T, valSet *ValidatorSet) {
	// verify that the vals' tvp is set to the sum of the all vals voting powers
	tvp := valSet.TotalVotingPower()
	assert.Equal(t, valSet.totalVotingPower, tvp,
		"expected TVP %d. Got %d, valSet=%s", tvp, valSet.totalVotingPower, valSet)

	// verify that validator priorities are centered
	l := int64(len(valSet.Validators))
	tpp := valSet.TotalVotingPower()
	assert.True(t, tpp <= l || tpp >= -l,
		"expected total priority in (-%d, %d). Got %d", l, l, tpp)

	// verify that priorities are scaled
	dist := computeMaxMinPriorityDiff(valSet)
	assert.True(t, dist <= PriorityWindowSizeFactor*tvp,
		"expected priority distance < %d. Got %d", PriorityWindowSizeFactor*tvp, dist)
}

func BenchmarkUpdates(b *testing.B) {
	const (
		n = 100
		m = 2000
	)
	// Init with n validators
	vs := make([]*Validator, n)
	for j := 0; j < n; j++ {
		vs[j] = newValidator([]byte(fmt.Sprintf("v%d", j)), 100)
	}
	valSet := NewValidatorSet(vs)
	l := len(valSet.Validators)

	// Make m new validators
	newValList := make([]*Validator, m)
	for j := 0; j < m; j++ {
		newValList[j] = newValidator([]byte(fmt.Sprintf("v%d", j+l)), 1000)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Add m validators to valSetCopy
		valSetCopy := valSet.Copy()
		assert.NoError(b, valSetCopy.UpdateWithChangeSet(newValList))
	}
}
