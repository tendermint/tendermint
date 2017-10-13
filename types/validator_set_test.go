package types

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

func randPubKey() crypto.PubKey {
	var pubKey [32]byte
	copy(pubKey[:], cmn.RandBytes(32))
	return crypto.PubKeyEd25519(pubKey).Wrap()
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
		b := vset.ToBytes()
		vset.FromBytes(b)

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

func TestFuzzValidatorSetFromBytes(t *testing.T) {
	// See Issue https://github.com/tendermint/tendermint/issues/722
	// which was fixed by https://github.com/tendermint/go-wire/pull/38
	vectors := [][]byte{
		{
			0x01, 0x01, 0x30, 0x01, 0x06, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30,
		},
		{
			0x01, 0x01, 0x02, 0x01, 0x01, 0x14, 0x5e, 0xbe, 0x09, 0x30, 0x40, 0xbe, 0x46, 0x15, 0x96, 0x0a,
		},
		{
			0x01, 0x01, 0x03, 0x01, 0x01, 0x14, 0x89, 0xca, 0x3e, 0xfb, 0x67, 0x05, 0x07, 0xa7, 0x9d, 0xaf,
			0xd0, 0x24, 0xdd, 0x8d, 0x11, 0xd0, 0x56, 0x16, 0xb3, 0x28, 0x01, 0xd3, 0xab, 0x48, 0xc9, 0x9a,
			0xbc, 0x8f, 0x23, 0x93, 0xe8, 0xd3, 0x4c, 0x5c, 0x14, 0x41, 0x53, 0xaa, 0x46, 0x4e, 0x92, 0xf9,
			0x81, 0xb7, 0xf0, 0x72, 0x8c, 0xaf, 0xae, 0xb5, 0x66, 0xe1, 0x25, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x64, 0xf2, 0x25, 0x07, 0x5d, 0x00, 0x00, 0x00, 0x64, 0x01, 0x01, 0x14, 0xe1, 0x56,
			0x43, 0x9c, 0x68, 0x5a, 0x37, 0xf1, 0x75, 0x91, 0xa4, 0xac, 0xc4, 0x2a, 0xb3, 0xbd, 0x4a, 0x0a,
			0xeb, 0xce, 0x01, 0x69, 0xf7, 0x30, 0xbb, 0xe1, 0x4e, 0x49, 0x71, 0xa0, 0xfd, 0x14, 0x7a, 0x0f,
			0xeb, 0x1a, 0xa7, 0xd9, 0x4d, 0xae, 0x8b, 0x1a, 0x08, 0x20, 0x71, 0xab, 0x7f, 0x11, 0x16, 0x77,
			0x24, 0xbb, 0xb7, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x64, 0x01, 0x01, 0x14, 0xf2, 0x55, 0x75, 0x8f, 0xc2, 0x91, 0xb1, 0xaf, 0x74, 0x5c,
			0x45, 0x78, 0x00, 0x03, 0x54, 0x65, 0xb1, 0x23, 0x22, 0x6a, 0x01, 0x88, 0xa6, 0xae, 0xf2, 0x25,
			0x07, 0x5d, 0x1b, 0x7f, 0x9f, 0x2c, 0x77, 0x30, 0x2a, 0xf7, 0x8f, 0x50, 0x3b, 0x60, 0xc4, 0x41,
			0xd3, 0x11, 0x32, 0x65, 0x8a, 0xa9, 0x13, 0xd7, 0xe1, 0x0b, 0xc3, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x01, 0xf4, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x38, 0x01, 0x05, 0x14, 0xf2, 0x55,
			0x75, 0x8f, 0xc2, 0x91, 0xb1, 0xaf, 0x74, 0x5c, 0x45, 0x78, 0x00, 0x03, 0x54, 0x65, 0xb1, 0x23,
			0x22, 0x6a, 0x01, 0x88, 0xa6, 0xae, 0xf2, 0x25, 0x07, 0x5d, 0x1b, 0x7f, 0x9f, 0x2c, 0x77, 0x30,
			0x2a, 0xf7, 0x8f, 0x50, 0x3b, 0x60, 0xc4, 0x41, 0xd3, 0x11, 0x32, 0x65, 0x8a, 0xa9, 0x13, 0xd7,
			0xe1, 0x0b, 0xc3, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xf4, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0x38,
		},
	}

	for i, malicious := range vectors {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("#%d: expected a panic\ndata: % x", i, malicious)
					return
				}
				// Expecting either just EOF or Overflow, not a runtime panic or anything else
				var err error
				switch typ := r.(type) {
				case error:
					err = typ
				default:
					err = fmt.Errorf("%v", typ)
				}
				ok := strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "verflow")
				if !ok {
					t.Errorf(`#%d: got: %v\nexpecting to contain either "EOF" or "overflow"\ndata: % x`, i, err, malicious)
				}
			}()
			vs := new(ValidatorSet)
			vs.FromBytes(malicious)
		}()
	}
}
