package types

import (
	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"

	"bytes"
	"strings"
	"testing"
)

func randPubKey() account.PubKeyEd25519 {
	var pubKey [32]byte
	copy(pubKey[:], RandBytes(32))
	return account.PubKeyEd25519(pubKey)
}

func randValidator_() *Validator {
	return &Validator{
		Address:     RandBytes(20),
		PubKey:      randPubKey(),
		BondHeight:  RandInt(),
		VotingPower: RandInt64(),
		Accum:       RandInt64(),
	}
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
		&Validator{
			Address:     []byte("foo"),
			PubKey:      randPubKey(),
			BondHeight:  RandInt(),
			VotingPower: 1000,
			Accum:       0,
		},
		&Validator{
			Address:     []byte("bar"),
			PubKey:      randPubKey(),
			BondHeight:  RandInt(),
			VotingPower: 300,
			Accum:       0,
		},
		&Validator{
			Address:     []byte("baz"),
			PubKey:      randPubKey(),
			BondHeight:  RandInt(),
			VotingPower: 330,
			Accum:       0,
		},
	})
	proposers := []string{}
	for i := 0; i < 100; i++ {
		val := vset.Proposer()
		proposers = append(proposers, string(val.Address))
		vset.IncrementAccum(1)
	}
	expected := `bar foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo foo baz bar foo foo foo baz foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo bar foo foo baz foo foo bar foo baz foo foo bar foo baz foo foo bar foo baz foo foo`
	if expected != strings.Join(proposers, " ") {
		t.Errorf("Expected sequence of proposers was\n%v\nbut got \n%v", expected, strings.Join(proposers, " "))
	}
}

func BenchmarkValidatorSetCopy(b *testing.B) {
	b.StopTimer()
	vset := NewValidatorSet([]*Validator{})
	for i := 0; i < 1000; i++ {
		privAccount := account.GenPrivAccount()
		val := &Validator{
			Address: privAccount.Address,
			PubKey:  privAccount.PubKey.(account.PubKeyEd25519),
		}
		if !vset.Add(val) {
			panic("Failed to add validator")
		}
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		vset.Copy()
	}
}
