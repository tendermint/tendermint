package types

import (
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"

	"bytes"
	"strings"
	"testing"
)

func randPubKey() crypto.PubKeyEd25519 {
	var pubKey [32]byte
	copy(pubKey[:], RandBytes(32))
	return crypto.PubKeyEd25519(pubKey)
}

func randValidator_() *Validator {
	val := NewValidator(randPubKey(), RandInt64())
	val.Accum = RandInt64()
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

func TestProposerSelection(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		newValidator([]byte("foo"), 1000),
		newValidator([]byte("bar"), 300),
		newValidator([]byte("baz"), 330),
	})
	proposers := []string{}
	for i := 0; i < 99; i++ {
		val := vset.Proposer()
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
	addr1 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	addr2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	addr3 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}

	// when all voting power is same, we go in order of addresses
	val1, val2, val3 := newValidator(addr1, 100), newValidator(addr2, 100), newValidator(addr3, 100)
	valList := []*Validator{val1, val2, val3}
	vals := NewValidatorSet(valList)
	for i := 0; i < len(valList)*5; i++ {
		ii := i % len(valList)
		prop := vals.Proposer()
		if !bytes.Equal(prop.Address, valList[ii].Address) {
			t.Fatalf("Expected %X. Got %X", valList[ii].Address, prop.Address)
		}
		vals.IncrementAccum(1)
	}

	// One validator has more than the others, but not enough to propose twice in a row
	*val3 = *newValidator(addr3, 400)
	vals = NewValidatorSet(valList)
	prop := vals.Proposer()
	if !bytes.Equal(prop.Address, addr3) {
		t.Fatalf("Expected address with highest voting power to be first proposer. Got %X", prop.Address)
	}
	vals.IncrementAccum(1)
	prop = vals.Proposer()
	if !bytes.Equal(prop.Address, addr1) {
		t.Fatalf("Expected smallest address to be validator. Got %X", prop.Address)
	}

	// One validator has more than the others, and enough to be proposer twice in a row
	*val3 = *newValidator(addr3, 401)
	vals = NewValidatorSet(valList)
	prop = vals.Proposer()
	if !bytes.Equal(prop.Address, addr3) {
		t.Fatalf("Expected address with highest voting power to be first proposer. Got %X", prop.Address)
	}
	vals.IncrementAccum(1)
	prop = vals.Proposer()
	if !bytes.Equal(prop.Address, addr3) {
		t.Fatalf("Expected address with highest voting power to be second proposer. Got %X", prop.Address)
	}
	vals.IncrementAccum(1)
	prop = vals.Proposer()
	if !bytes.Equal(prop.Address, addr1) {
		t.Fatalf("Expected smallest address to be validator. Got %X", prop.Address)
	}

	// each validator should be the proposer a proportional number of times
	val1, val2, val3 = newValidator(addr1, 4), newValidator(addr2, 5), newValidator(addr3, 3)
	valList = []*Validator{val1, val2, val3}
	propCount := make([]int, 3)
	vals = NewValidatorSet(valList)
	for i := 0; i < 120; i++ {
		prop := vals.Proposer()
		ii := prop.Address[19]
		propCount[ii] += 1
		vals.IncrementAccum(1)
	}

	if propCount[0] != 40 {
		t.Fatalf("Expected prop count for validator with 4/12 of voting power to be 40/120. Got %d/120", propCount[0])
	}
	if propCount[1] != 50 {
		t.Fatalf("Expected prop count for validator with 5/12 of voting power to be 50/120. Got %d/120", propCount[1])
	}
	if propCount[2] != 30 {
		t.Fatalf("Expected prop count for validator with 3/12 of voting power to be 30/120. Got %d/120", propCount[2])
	}
}

func BenchmarkValidatorSetCopy(b *testing.B) {
	b.StopTimer()
	vset := NewValidatorSet([]*Validator{})
	for i := 0; i < 1000; i++ {
		privKey := crypto.GenPrivKeyEd25519()
		pubKey := privKey.PubKey().(crypto.PubKeyEd25519)
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
