package types

import (
	"bytes"
	"fmt"
	"math"
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
	assert.True(t, vset.Add(val))
	assert.True(t, vset.HasAddress(val.Address))
	idx, val2 := vset.GetByAddress(val.Address)
	assert.Equal(t, 0, idx)
	assert.Equal(t, val, val2)
	addr, val2 = vset.GetByIndex(0)
	assert.Equal(t, []byte(val.Address), addr)
	assert.Equal(t, val, val2)
	assert.Equal(t, 1, vset.Size())
	assert.Equal(t, val.VotingPower, vset.TotalVotingPower())
	assert.Equal(t, val, vset.GetProposer())
	assert.NotNil(t, vset.Hash())
	assert.NotPanics(t, func() { vset.IncrementProposerPriority(1) })

	// update
	assert.False(t, vset.Update(randValidator_(vset.TotalVotingPower())))
	_, val = vset.GetByAddress(val.Address)
	val.VotingPower += 100
	proposerPriority := val.ProposerPriority
	// Mimic update from types.PB2TM.ValidatorUpdates which does not know about ProposerPriority
	// and hence defaults to 0.
	val.ProposerPriority = 0
	assert.True(t, vset.Update(val))
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
		val := NewValidator(pubKey, 0)
		if !vset.Add(val) {
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
	val := NewValidator(randPubKey(), cmn.RandInt64()%(MaxTotalVotingPower-totalVotingPower))
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
	vals := ValidatorSet{Validators: []*Validator{
		{Address: []byte{0}, ProposerPriority: 0, VotingPower: 10},
		{Address: []byte{1}, ProposerPriority: 0, VotingPower: 1},
		{Address: []byte{2}, ProposerPriority: 0, VotingPower: 1}}}
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
				0 + 10 - 12 - 4, // mostest will be subtracted by total voting power (12)
				0 + 1 - 4,
				0 + 1 - 4},
			1,
			vals.Validators[0]},
		1: {
			vals.Copy(),
			[]int64{
				(0 + 10 - 12 - 4) + 10 - 12 + 4, // this will be mostest on 2nd iter, too
				(0 + 1 - 4) + 1 + 4,
				(0 + 1 - 4) + 1 + 4},
			2,
			vals.Validators[0]}, // increment twice -> expect average to be subtracted twice
		2: {
			vals.Copy(),
			[]int64{
				((0 + 10 - 12 - 4) + 10 - 12) + 10 - 12 + 4, // still mostest
				((0 + 1 - 4) + 1) + 1 + 4,
				((0 + 1 - 4) + 1) + 1 + 4},
			3,
			vals.Validators[0]},
		3: {
			vals.Copy(),
			[]int64{
				0 + 4*(10-12) + 4 - 4, // still mostest
				0 + 4*1 + 4 - 4,
				0 + 4*1 + 4 - 4},
			4,
			vals.Validators[0]},
		4: {
			vals.Copy(),
			[]int64{
				0 + 4*(10-12) + 10 + 4 - 4, // 4 iters was mostest
				0 + 5*1 - 12 + 4 - 4,       // now this val is mostest for the 1st time (hence -12==totalVotingPower)
				0 + 5*1 + 4 - 4},
			5,
			vals.Validators[1]},
		5: {
			vals.Copy(),
			[]int64{
				0 + 6*10 - 5*12 + 4 - 4, // mostest again
				0 + 6*1 - 12 + 4 - 4,    // mostest once up to here
				0 + 6*1 + 4 - 4},
			6,
			vals.Validators[0]},
		6: {
			vals.Copy(),
			[]int64{
				0 + 7*10 - 6*12 + 4 - 4, // in 7 iters this val is mostest 6 times
				0 + 7*1 - 12 + 4 - 4,    // in 7 iters this val is mostest 1 time
				0 + 7*1 + 4 - 4},
			7,
			vals.Validators[0]},
		7: {
			vals.Copy(),
			[]int64{
				0 + 8*10 - 7*12 + 4 - 4, // mostest
				0 + 8*1 - 12 + 4 - 4,
				0 + 8*1 + 4 - 4},
			8,
			vals.Validators[0]},
		8: {
			vals.Copy(),
			[]int64{
				0 + 9*10 - 7*12 + 4 - 4,
				0 + 9*1 - 12 + 4 - 4,
				0 + 9*1 - 12 + 4 - 4}, // mostest
			9,
			vals.Validators[2]},
		9: {
			vals.Copy(),
			[]int64{
				0 + 10*10 - 8*12 + 4 - 4, // after 10 iters this is mostest again
				0 + 10*1 - 12 + 4 - 4,    // after 6 iters this val is "mostest" once and not in between
				0 + 10*1 - 12 + 4 - 4},   // in between 10 iters this val is "mostest" once
			10,
			vals.Validators[0]},
		10: {
			vals.Copy(),
			[]int64{
				// shift twice inside incrementProposerPriority (shift every 10th iter);
				// don't shift at the end of IncremenctProposerPriority
				// last avg should be zero because
				// ProposerPriority of validator 0: (0 + 11*10 - 8*12 - 4) == 10
				// ProposerPriority of validator 1 and 2: (0 + 11*1 - 12 - 4) == -5
				// and (10 + 5 - 5) / 3 == 0
				0 + 11*10 - 8*12 - 4 - 12 - 0,
				0 + 11*1 - 12 - 4 - 0,  // after 6 iters this val is "mostest" once and not in between
				0 + 11*1 - 12 - 4 - 0}, // after 10 iters this val is "mostest" once
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
	commit := &Commit{
		BlockID:    blockID,
		Precommits: []*Vote{vote},
	}

	badChainID := "notmychainID"
	badBlockID := BlockID{Hash: []byte("goodbye")}
	badHeight := height + 1
	badCommit := &Commit{
		BlockID:    blockID,
		Precommits: []*Vote{nil},
	}

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
