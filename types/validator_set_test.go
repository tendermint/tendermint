package types

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/tendermint/wire"
	cmn "github.com/tendermint/tmlibs/common"
)

func randPubKey() crypto.PubKey {
	var pubKey [32]byte
	copy(pubKey[:], cmn.RandBytes(32))
	return crypto.PubKeyEd25519(pubKey)
}

func randValidator_() *Validator {
	val := NewValidator(randPubKey(), cmn.RandInt64())
	val.Accum = cmn.RandInt64()
	return val
}

func randValidatorSet(numValidators int) *ValidatorSet {
	validators := make([]*Validator, numValidators)
	for i := 0; i < numValidators; i++ {
		validators[i] = randValidator_()
	}
	return NewValidatorSet(validators)
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

func TestProposerSelection1(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		newValidator([]byte("foo"), 1000),
		newValidator([]byte("bar"), 300),
		newValidator([]byte("baz"), 330),
	})
	proposers := []string{}
	for i := 0; i < 99; i++ {
		val := vset.GetProposer()
		proposers = append(proposers, string(val.Address))
		vset.IncrementAccum(1)
	}
	expected := `foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo foo baz bar foo foo foo baz foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo`
	if expected != strings.Join(proposers, " ") {
		t.Errorf("Expected sequence of proposers was\n%v\nbut got \n%v", expected, strings.Join(proposers, " "))
	}
}

func newValidator(address []byte, power int64) *Validator {
	return &Validator{Address: address, VotingPower: power}
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
		vals.IncrementAccum(1)
	}

	// One validator has more than the others, but not enough to propose twice in a row
	*val2 = *newValidator(addr2, 400)
	vals = NewValidatorSet(valList)
	// vals.IncrementAccum(1)
	prop := vals.GetProposer()
	if !bytes.Equal(prop.Address, addr2) {
		t.Fatalf("Expected address with highest voting power to be first proposer. Got %X", prop.Address)
	}
	vals.IncrementAccum(1)
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
	vals.IncrementAccum(1)
	prop = vals.GetProposer()
	if !bytes.Equal(prop.Address, addr2) {
		t.Fatalf("Expected address with highest voting power to be second proposer. Got %X", prop.Address)
	}
	vals.IncrementAccum(1)
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
		propCount[ii] += 1
		vals.IncrementAccum(1)
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
		vset.IncrementAccum(1)
	}

	// i for the loop
	// j for the times
	// we should go in order for ever, despite some IncrementAccums with times > 1
	var i, j int
	for ; i < 10000; i++ {
		got := vset.GetProposer().Address
		expected := proposerOrder[j%4].Address
		if !bytes.Equal(got, expected) {
			t.Fatalf(cmn.Fmt("vset.Proposer (%X) does not match expected proposer (%X) for (%d, %d)", got, expected, i, j))
		}

		// serialize, deserialize, check proposer
		b := vset.toBytes()
		vset.fromBytes(b)

		computed := vset.GetProposer() // findGetProposer()
		if i != 0 {
			if !bytes.Equal(got, computed.Address) {
				t.Fatalf(cmn.Fmt("vset.Proposer (%X) does not match computed proposer (%X) for (%d, %d)", got, computed.Address, i, j))
			}
		}

		// times is usually 1
		times := 1
		mod := (cmn.RandInt() % 5) + 1
		if cmn.RandInt()%mod > 0 {
			// sometimes its up to 5
			times = cmn.RandInt() % 5
		}
		vset.IncrementAccum(times)

		j += times
	}
}

func TestValidatorSetTotalVotingPowerOverflows(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		{Address: []byte("a"), VotingPower: math.MaxInt64, Accum: 0},
		{Address: []byte("b"), VotingPower: math.MaxInt64, Accum: 0},
		{Address: []byte("c"), VotingPower: math.MaxInt64, Accum: 0},
	})

	assert.EqualValues(t, math.MaxInt64, vset.TotalVotingPower())
}

func TestValidatorSetIncrementAccumOverflows(t *testing.T) {
	// NewValidatorSet calls IncrementAccum(1)
	vset := NewValidatorSet([]*Validator{
		// too much voting power
		0: {Address: []byte("a"), VotingPower: math.MaxInt64, Accum: 0},
		// too big accum
		1: {Address: []byte("b"), VotingPower: 10, Accum: math.MaxInt64},
		// almost too big accum
		2: {Address: []byte("c"), VotingPower: 10, Accum: math.MaxInt64 - 5},
	})

	assert.Equal(t, int64(0), vset.Validators[0].Accum, "0") // because we decrement val with most voting power
	assert.EqualValues(t, math.MaxInt64, vset.Validators[1].Accum, "1")
	assert.EqualValues(t, math.MaxInt64, vset.Validators[2].Accum, "2")
}

func TestValidatorSetIncrementAccumUnderflows(t *testing.T) {
	// NewValidatorSet calls IncrementAccum(1)
	vset := NewValidatorSet([]*Validator{
		0: {Address: []byte("a"), VotingPower: math.MaxInt64, Accum: math.MinInt64},
		1: {Address: []byte("b"), VotingPower: 1, Accum: math.MinInt64},
	})

	vset.IncrementAccum(5)

	assert.EqualValues(t, math.MinInt64, vset.Validators[0].Accum, "0")
	assert.EqualValues(t, math.MinInt64, vset.Validators[1].Accum, "1")
}

func TestSafeMul(t *testing.T) {
	f := func(a, b int64) bool {
		c, overflow := safeMul(a, b)
		return overflow || (!overflow && c == a*b)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
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

func TestSafeMulClip(t *testing.T) {
	assert.EqualValues(t, math.MaxInt64, safeMulClip(math.MinInt64, math.MinInt64))
	assert.EqualValues(t, math.MinInt64, safeMulClip(math.MaxInt64, math.MinInt64))
	assert.EqualValues(t, math.MinInt64, safeMulClip(math.MinInt64, math.MaxInt64))
	assert.EqualValues(t, math.MaxInt64, safeMulClip(math.MaxInt64, 2))
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

func BenchmarkValidatorSetCopy(b *testing.B) {
	b.StopTimer()
	vset := NewValidatorSet([]*Validator{})
	for i := 0; i < 1000; i++ {
		privKey := crypto.GenPrivKeyEd25519()
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

func (valSet *ValidatorSet) toBytes() []byte {
	bz, err := wire.MarshalBinary(valSet)
	if err != nil {
		panic(err)
	}
	return bz
}

func (valSet *ValidatorSet) fromBytes(b []byte) {
	err := wire.UnmarshalBinary(b, valSet)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		panic(err)
	}
}
