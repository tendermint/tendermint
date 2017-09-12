package types

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	cmn "github.com/tendermint/tmlibs/common"

	"github.com/tendermint/go-crypto"
)

func TestVerifyCommit(t *testing.T) {
	assert := assert.New(t)
	keys := genValKeys(4)
	// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do
	vals := keys.toValidators(20, 10)
	chainID := "test-verify"

	cases := []struct {
		keys        valKeys
		vals        *ValidatorSet
		height      int
		first, last int  // who actually signs
		proper      bool // true -> expect no error
	}{
		// perfect, signed by everyone
		{keys, vals, 1, 0, len(keys), true},
		// skipping the smallest is okay
		{keys, vals, 2, 1, len(keys), true},
		// but skipping the largest isn't
		{keys, vals, 3, 0, len(keys) - 1, false},
	}

	for _, tc := range cases {
		commit, header := tc.keys.genCommit(chainID, tc.height, nil, tc.vals,
			[]byte("bar"), tc.first, tc.last)
		blockID := BlockID{Hash: header.Hash()}
		err := tc.vals.VerifyCommit(chainID, blockID, tc.height, commit)
		if tc.proper {
			assert.Nil(err, "%+v", err)
		} else {
			assert.NotNil(err)
		}
	}
}

func TestVerifyCommitAny(t *testing.T) {
	assert := assert.New(t)
	keys := genValKeys(4)

	// TODO: Upgrade the helper function to better facilitate this test
	// Create a new validator with very high voting power
	// Tests whether VerifyCommitAny will reject a commit that has not been signed
	// by more than 2/3 of the new validator set.
	val := NewValidator(randPubKey(), 1000)
	val.Accum = cmn.RandInt64()

	// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
	vals := keys.toValidators(20, 10)

	newVals := vals.Copy()
	newVals.Add(val)

	chainID := "test-verify"

	cases := []struct {
		keys        valKeys
		oldVals     *ValidatorSet
		newVals     *ValidatorSet
		height      int
		proper      bool // true -> expect no error
	}{
		// perfect, signed by everyone in both sets
		{keys, vals, vals, 1, true},
		// no majority from newVals
		{keys, vals, newVals, 2, false},
	}

	for _, tc := range cases {
		commit, header := tc.keys.genCommit(chainID, tc.height, nil, tc.oldVals,
			[]byte("bar"), 0, len(tc.keys))
		blockID := BlockID{Hash: header.Hash()}
		err := tc.oldVals.VerifyCommitAny(tc.newVals, chainID, blockID, tc.height, commit)
		if tc.proper {
			assert.Nil(err, "%+v", err)
		} else {
			assert.NotNil(err)
		}
	}
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

// -----------------------------------------------------------------------------
// Helpers

// Test Helper: ValKeys lets us simulate signing with many keys
type valKeys []crypto.PrivKey

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

// genValKeys produces an array of private keys to generate commits
func genValKeys(n int) valKeys {
	res := make(valKeys, n)
	for i := range res {
		res[i] = crypto.GenPrivKeyEd25519().Wrap()
	}
	return res
}

// toValidators produces a list of validators from the set of keys
// The first key has weight `init` and it increases by `inc` every step
// so we can have all the same weight, or a simple linear distribution
func (v valKeys) toValidators(init, inc int64) *ValidatorSet {
	res := make([]*Validator, len(v))
	for i, k := range v {
		res[i] = NewValidator(k.PubKey(), init+int64(i)*inc)
	}
	return NewValidatorSet(res)
}

func genHeader(chainID string, height int, txs Txs,
	vals *ValidatorSet, appHash []byte) *Header {

	return &Header{
		ChainID: chainID,
		Height:  height,
		Time:    time.Now(),
		NumTxs:  len(txs),
		ValidatorsHash: vals.Hash(),
		DataHash:       txs.Hash(),
		AppHash:        appHash,
	}
}

// signHeader properly signs the header with all keys from first to last inclusive
func (v valKeys) signHeader(header *Header, first, last int) *Commit {
	votes := make([]*Vote, len(v))

	// we need this list to keep the ordering...
	vset := v.toValidators(1, 0)

	// fill in the votes we want
	for i := first; i < last; i++ {
		vote := makeVote(header, vset, v[i])
		votes[vote.ValidatorIndex] = vote
	}

	res := &Commit{
		BlockID:    BlockID{Hash: header.Hash()},
		Precommits: votes,
	}
	return res
}

func makeVote(header *Header, vals *ValidatorSet, key crypto.PrivKey) *Vote {
	addr := key.PubKey().Address()
	idx, _ := vals.GetByAddress(addr)
	vote := &Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           header.Height,
		Round:            1,
		Type:             VoteTypePrecommit,
		BlockID:          BlockID{Hash: header.Hash()},
	}
	// Sign it
	signBytes := SignBytes(header.ChainID, vote)
	vote.Signature = key.Sign(signBytes)
	return vote
}

// genCommit calls genHeader and signHeader
func (v valKeys) genCommit(chainID string, height int, txs Txs,
	vals *ValidatorSet, appHash []byte, first, last int) (*Commit, *Header) {

	header := genHeader(chainID, height, txs, vals, appHash)
	return v.signHeader(header, first, last), header
}
