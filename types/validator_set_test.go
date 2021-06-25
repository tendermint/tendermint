//nolint: lll
package types

import (
	"bytes"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"math"
	"sort"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestValidatorSetBasic(t *testing.T) {
	// empty or nil validator lists are allowed,
	// but attempting to IncrementProposerPriority on them will panic.
	vset := NewValidatorSet([]*Validator{}, nil, btcjson.LLMQType_5_60, nil, true)
	assert.Panics(t, func() { vset.IncrementProposerPriority(1) })

	vset = NewValidatorSet(nil, nil, btcjson.LLMQType_5_60, nil, true)
	assert.Panics(t, func() { vset.IncrementProposerPriority(1) })

	assert.EqualValues(t, vset, vset.Copy())
	assert.False(t, vset.HasProTxHash([]byte("some val")))
	idx, val := vset.GetByProTxHash([]byte("some val"))
	assert.EqualValues(t, -1, idx)
	assert.Nil(t, val)
	proTxHash, val := vset.GetByIndex(-100)
	assert.Nil(t, proTxHash)
	assert.Nil(t, val)
	proTxHash, val = vset.GetByIndex(0)
	assert.Nil(t, proTxHash)
	assert.Nil(t, val)
	proTxHash, val = vset.GetByIndex(100)
	assert.Nil(t, proTxHash)
	assert.Nil(t, val)
	assert.Zero(t, vset.Size())
	assert.Equal(t, int64(0), vset.TotalVotingPower())
	assert.Nil(t, vset.GetProposer())
	assert.Equal(t, []byte(nil), vset.Hash())
	// add
	val = randValidator(vset.TotalVotingPower())
	assert.NoError(t, vset.UpdateWithChangeSet([]*Validator{val}, val.PubKey, crypto.RandQuorumHash()))

	assert.True(t, vset.HasProTxHash(val.ProTxHash))
	idx, _ = vset.GetByProTxHash(val.ProTxHash)
	assert.EqualValues(t, 0, idx)
	proTxHash, _ = vset.GetByIndex(0)
	assert.Equal(t, val.ProTxHash, proTxHash)
	assert.Equal(t, 1, vset.Size())
	assert.Equal(t, val.VotingPower, vset.TotalVotingPower())
	assert.NotNil(t, vset.Hash())
	assert.NotPanics(t, func() { vset.IncrementProposerPriority(1) })
	assert.Equal(t, val.ProTxHash, vset.GetProposer().ProTxHash)

	// update
	val = randValidator(vset.TotalVotingPower())
	assert.NoError(t, vset.UpdateWithChangeSet([]*Validator{val}, val.PubKey, crypto.RandQuorumHash()))
	_, val = vset.GetByProTxHash(val.ProTxHash)
	val.PubKey = bls12381.GenPrivKey().PubKey()
	proposerPriority := val.ProposerPriority

	val.ProposerPriority = 0
	assert.NoError(t, vset.UpdateWithChangeSet([]*Validator{val}, val.PubKey, crypto.RandQuorumHash()))
	_, val = vset.GetByProTxHash(val.ProTxHash)
	assert.Equal(t, proposerPriority, val.ProposerPriority)

}

func TestValidatorSetValidateBasic(t *testing.T) {
	val, _ := RandValidator()
	badValNoPublicKey := &Validator{ProTxHash: val.ProTxHash}
	badValNoProTxHash := &Validator{PubKey: val.PubKey}

	goodValSet, _ := GenerateValidatorSet(4)
	badValSet, _ := GenerateValidatorSet(4)
	badValSet.ThresholdPublicKey = val.PubKey

	testCases := []struct {
		testName string
		vals ValidatorSet
		err  bool
		msg  string
	}{
		{
			testName: "Validator set needs members even with threshold public key",
			vals: ValidatorSet{ThresholdPublicKey: bls12381.GenPrivKey().PubKey()},
			err:  true,
			msg:  "validator set is nil or empty",
		},
		{
			testName: "Validator set needs members",
			vals: ValidatorSet{},
			err:  true,
			msg:  "validator set is nil or empty",
		},
		{
			testName: "Validator set needs members even with threshold public key and quorum hash",
			vals: ValidatorSet{
				Validators:         []*Validator{},
				ThresholdPublicKey: bls12381.GenPrivKey().PubKey(),
				QuorumHash:         crypto.RandQuorumHash(),
				HasPublicKeys:      true,
			},
			err: true,
			msg: "validator set is nil or empty",
		},
		{
			testName: "Validator set needs members even with quorum hash",
			vals: ValidatorSet{
				Validators: []*Validator{},
				QuorumHash: crypto.RandQuorumHash(),
				HasPublicKeys:      true,
			},
			err: true,
			msg: "validator set is nil or empty",
		},
		{
			testName: "Validator set needs members even with quorum hash",
			vals: ValidatorSet{
				Validators:         []*Validator{val},
				ThresholdPublicKey: val.PubKey,
				QuorumHash:         crypto.RandQuorumHash(),
				HasPublicKeys:      true,
			},
			err: true,
			msg: "validator set proposer is not set",
		},
		{
			testName: "Validator in set has wrong public key for threshold",
			vals: ValidatorSet{
				Validators:         []*Validator{val},
				ThresholdPublicKey: bls12381.GenPrivKey().PubKey(),
				QuorumHash:         crypto.RandQuorumHash(),
				HasPublicKeys:      true,
				Proposer: val,
			},
			err: true,
			msg: "thresholdPublicKey error: incorrect threshold public key",
		},
		{
			testName: "Validator in set has no public key",
			vals: ValidatorSet{
				Validators:         []*Validator{badValNoPublicKey},
				ThresholdPublicKey: bls12381.GenPrivKey().PubKey(),
				QuorumHash:         crypto.RandQuorumHash(),
				HasPublicKeys:      true,
				Proposer:           badValNoPublicKey,
			},
			err: true,
			msg: "invalid validator pub key #0: validator does not have a public key",
		},
		{
			testName: "Validator in set has no proTxHash",
			vals: ValidatorSet{
				Validators:         []*Validator{badValNoProTxHash},
				ThresholdPublicKey: bls12381.GenPrivKey().PubKey(),
				QuorumHash:         crypto.RandQuorumHash(),
				HasPublicKeys:      true,
				Proposer:           badValNoProTxHash,
			},
			err: true,
			msg: "invalid validator #0: validator does not have a provider transaction hash",
		},
		{
			testName: "Validator set needs quorum hash",
			vals: ValidatorSet{
				Validators:         []*Validator{val},
				Proposer:           val,
				ThresholdPublicKey: val.PubKey,
				HasPublicKeys:      true,
			},
			err: true,
			msg: "quorumHash error: quorum hash is not set",
		},
		{
			testName: "Validator set single val good",
			vals: ValidatorSet{
				Validators:         []*Validator{val},
				Proposer:           val,
				ThresholdPublicKey: val.PubKey,
				QuorumHash:         crypto.RandQuorumHash(),
				HasPublicKeys:      true,
			},
			err: false,
			msg: "",
		},
		{
			testName: "Validator set needs threshold public key",
			vals: ValidatorSet{
				Validators: []*Validator{val},
				Proposer:   val,
				QuorumHash:         crypto.RandQuorumHash(),
				HasPublicKeys:      true,
			},
			err: true,
			msg: "thresholdPublicKey error: threshold public key is not set",
		},
		{
			testName: "Validator set has incorrect threshold public key",
			vals: *badValSet,
			err:  true,
			msg:  "thresholdPublicKey error: incorrect recovered threshold public key",
		},
		{
			testName: "Validator set is good",
			vals: *goodValSet,
			err:  false,
			msg:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.vals.ValidateBasic()
			if tc.err {
				if assert.Error(t, err) {
					assert.True(t, strings.HasPrefix(err.Error(), tc.msg))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCopy(t *testing.T) {
	vset, _ := GenerateValidatorSet(10)
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
		NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("foo"))),
		NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("bar"))),
		NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("baz"))),
	}, pubKeyBLS{}, btcjson.LLMQType_5_60, crypto.QuorumHash{}, true)

	assert.Panics(t, func() { vset.IncrementProposerPriority(-1) })
	assert.Panics(t, func() { vset.IncrementProposerPriority(0) })
	vset.IncrementProposerPriority(1)
}

func BenchmarkValidatorSetCopy(b *testing.B) {
	b.StopTimer()
	vset := NewValidatorSet([]*Validator{}, nil, btcjson.LLMQType_5_60, nil, true)
	for i := 0; i < 1000; i++ {
		privKey := bls12381.GenPrivKey()
		pubKey := privKey.PubKey()
		val := NewValidatorDefaultVotingPower(pubKey, crypto.ProTxHash{})
		err := vset.UpdateWithChangeSet([]*Validator{val}, nil, crypto.RandQuorumHash())
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
	fooProTxHash := crypto.ProTxHash(crypto.Sha256([]byte("foo")))
	barProTxHash := crypto.ProTxHash(crypto.Sha256([]byte("bar")))
	bazProTxHash := crypto.ProTxHash(crypto.Sha256([]byte("baz")))
	vset := NewValidatorSet([]*Validator{
		NewTestValidatorGeneratedFromProTxHash(fooProTxHash),
		NewTestValidatorGeneratedFromProTxHash(barProTxHash),
		NewTestValidatorGeneratedFromProTxHash(bazProTxHash),
	}, pubKeyBLS{}, btcjson.LLMQType_5_60, crypto.QuorumHash{}, true)
	var proposers []string
	for i := 0; i < 99; i++ {
		val := vset.GetProposer()
		proposers = append(proposers, val.ProTxHash.ShortString())
		vset.IncrementProposerPriority(1)
	}
	expected := `2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B ` +
                `2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B ` +
		        `2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B ` +
		        `2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B ` +
		        `2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B ` +
		        `2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B ` +
                `2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B 2C26B4 BAA5A0 FCDE2B`
	if expected != strings.Join(proposers, " ") {
		t.Errorf("expected sequence of proposers was\n%v\nbut got \n%v", expected, strings.Join(proposers, " "))
	}
}

func TestProposerSelection2(t *testing.T) {
	proTxHashes := make([]crypto.ProTxHash, 3)
	addresses := make([]crypto.Address, 3)
	proTxHashes[0] = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	proTxHashes[1] = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
	proTxHashes[2] = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}
	addresses[0] = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
	addresses[1] = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	addresses[2] = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	vals, _ := GenerateValidatorSetUsingProTxHashes(proTxHashes)
	for i := 0; i < len(proTxHashes)*5; i++ {
		ii := (i) % len(proTxHashes)
		prop := vals.GetProposer()
		if !bytes.Equal(prop.ProTxHash, vals.Validators[ii].ProTxHash) {
			t.Fatalf("(%d): Expected %X. Got %X", i, vals.Validators[ii].ProTxHash, prop.ProTxHash)
		}
		vals.IncrementProposerPriority(1)
	}

	prop := vals.GetProposer()
	if !bytes.Equal(prop.ProTxHash, proTxHashes[0]) {
		t.Fatalf("Expected proposer with smallest pro_tx_hash to be first proposer. Got %X", prop.ProTxHash)
	}
	vals.IncrementProposerPriority(1)
	prop = vals.GetProposer()
	if !bytes.Equal(prop.ProTxHash, proTxHashes[1]) {
		t.Fatalf("Expected proposer with second smallest pro_tx_hash to be second proposer. Got %X", prop.ProTxHash)
	}
}

func TestProposerSelection3(t *testing.T) {
	proTxHashes := make([]crypto.ProTxHash, 4)
	proTxHashes[0] = crypto.Sha256([]byte("avalidator_address12"))
	proTxHashes[1] = crypto.Sha256([]byte("bvalidator_address12"))
	proTxHashes[2] = crypto.Sha256([]byte("cvalidator_address12"))
	proTxHashes[3] = crypto.Sha256([]byte("dvalidator_address12"))

	vset, _ := GenerateTestValidatorSetWithProTxHashesDefaultPower(proTxHashes)

	proposerOrder := make([]*Validator, 4)
	for i := 0; i < 4; i++ {
		proposerOrder[i] = vset.GetProposer()
		vset.IncrementProposerPriority(1)
	}

	// i for the loop
	// j for the times
	// we should go in order for ever, despite some IncrementProposerPriority with times > 1
	var (
		i int
		j int32
	)
	for ; i < 10000; i++ {
		got := vset.GetProposer().ProTxHash
		expected := proposerOrder[j%4].ProTxHash
		if !bytes.Equal(got, expected) {
			t.Fatalf(fmt.Sprintf("vset.Proposer (%X) does not match expected proposer (%X) for (%d, %d)", got, expected, i, j))
		}

		// serialize, deserialize, check proposer
		b := vset.toBytes()
		vset = vset.fromBytes(b)

		computed := vset.GetProposer() // findGetProposer()
		if i != 0 {
			if !bytes.Equal(got, computed.ProTxHash) {
				t.Fatalf(
					fmt.Sprintf(
						"vset.Proposer (%X) does not match computed proposer (%X) for (%d, %d)",
						got,
						computed.ProTxHash,
						i,
						j,
					),
				)
			}
		}

		// times is usually 1
		times := int32(1)
		mod := (tmrand.Int() % 5) + 1
		if tmrand.Int()%mod > 0 {
			// sometimes its up to 5
			times = (tmrand.Int31() % 4) + 1
		}
		vset.IncrementProposerPriority(times)

		j += times
	}
}

func randValidator(totalVotingPower int64) *Validator {
	// this modulo limits the ProposerPriority/VotingPower to stay in the
	// bounds of MaxTotalVotingPower minus the already existing voting power:
	val, _ := RandValidator()
	val.ProposerPriority = tmrand.Int64() % (MaxTotalVotingPower - totalVotingPower)
	return val
}

func (vals *ValidatorSet) toBytes() []byte {
	pbvs, err := vals.ToProto()
	if err != nil {
		panic(err)
	}

	bz, err := pbvs.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

func (vals *ValidatorSet) fromBytes(b []byte) *ValidatorSet {
	pbvs := new(tmproto.ValidatorSet)
	err := pbvs.Unmarshal(b)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		panic(err)
	}

	vs, err := ValidatorSetFromProto(pbvs)
	if err != nil {
		panic(err)
	}

	return vs
}

func TestAvgProposerPriority(t *testing.T) {
	// Create Validator set without calling IncrementProposerPriority:
	tcs := []struct {
		vs   ValidatorSet
		want int64
	}{
		0: {ValidatorSet{Validators: []*Validator{{ProposerPriority: 0}, {ProposerPriority: 0}, {ProposerPriority: 0}}}, 0},
		1: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: 0}, {ProposerPriority: 0}},
			}, math.MaxInt64 / 3,
		},
		2: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: 0}},
			}, math.MaxInt64 / 2,
		},
		3: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MaxInt64}, {ProposerPriority: math.MaxInt64}},
			}, math.MaxInt64,
		},
		4: {
			ValidatorSet{
				Validators: []*Validator{{ProposerPriority: math.MinInt64}, {ProposerPriority: math.MinInt64}},
			}, math.MinInt64,
		},
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
		times int32
		avg   int64
	}{
		0: {ValidatorSet{
			Validators: []*Validator{
				{ProTxHash: []byte("a"), ProposerPriority: 1},
				{ProTxHash: []byte("b"), ProposerPriority: 2},
				{ProTxHash: []byte("c"), ProposerPriority: 3}}},
			1, 2},
		1: {ValidatorSet{
			Validators: []*Validator{
				{ProTxHash: []byte("a"), ProposerPriority: 10},
				{ProTxHash: []byte("b"), ProposerPriority: -10},
				{ProTxHash: []byte("c"), ProposerPriority: 1}}},
			// this should average twice but the average should be 0 after the first iteration
			// (voting power is 0 -> no changes)
			11, 1 / 3},
		2: {ValidatorSet{
			Validators: []*Validator{
				{ProTxHash: []byte("a"), ProposerPriority: 100},
				{ProTxHash: []byte("b"), ProposerPriority: -10},
				{ProTxHash: []byte("c"), ProposerPriority: 1}}},
			1, 91 / 3},
	}
	for i, tc := range tcs {
		// work on copy to have the old ProposerPriorities:
		newVset := tc.vs.CopyIncrementProposerPriority(tc.times)
		for _, val := range tc.vs.Validators {
			_, updatedVal := newVset.GetByProTxHash(val.ProTxHash)
			assert.Equal(t, updatedVal.ProposerPriority, val.ProposerPriority-tc.avg, "test case: %v", i)
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

// Check VerifyCommit, VerifyCommitLight and VerifyCommitLightTrusting basic
// verification.
func TestValidatorSet_VerifyCommit_All(t *testing.T) {
	var (
		proTxHash  = crypto.RandProTxHash()
		privKey    = bls12381.GenPrivKey()
		pubKey     = privKey.PubKey()
		v1         = NewValidatorDefaultVotingPower(pubKey, proTxHash)
		quorumHash = crypto.RandQuorumHash()
		vset       = NewValidatorSet([]*Validator{v1}, v1.PubKey, btcjson.LLMQType_5_60, quorumHash, true)

		chainID = "Lalande21185"
	)

	vote := examplePrecommit()
	vote.ValidatorProTxHash = proTxHash
	v := vote.ToProto()
	blockSig, err := privKey.SignDigest(VoteBlockSignId(chainID, v, btcjson.LLMQType_5_60, quorumHash))
	require.NoError(t, err)
	stateSig, err := privKey.SignDigest(VoteStateSignId(chainID, v, btcjson.LLMQType_5_60, quorumHash))
	require.NoError(t, err)
	vote.BlockSignature = blockSig
	vote.StateSignature = stateSig

	commit := NewCommit(vote.Height,
		vote.Round,
		vote.BlockID,
		vote.StateID,
		quorumHash,
		vote.BlockSignature,
		vote.StateSignature,
	)

	vote2 := *vote
	blockSig2, err := privKey.SignDigest(VoteBlockSignBytes("EpsilonEridani", v))
	require.NoError(t, err)
	stateSig2, err := privKey.SignDigest(VoteStateSignBytes("EpsilonEridani", v))
	require.NoError(t, err)
	vote2.BlockSignature = blockSig2
	vote2.StateSignature = stateSig2

	testCases := []struct {
		description string
		chainID     string
		blockID     BlockID
		stateID     StateID
		height      int64
		commit      *Commit
		expErr      bool
	}{
		{"good", chainID, vote.BlockID, vote.StateID, vote.Height, commit, false},

		{"incorrect threshold block signature", "EpsilonEridani", vote.BlockID, vote.StateID, vote.Height, commit, true},
		{"wrong block ID", chainID, makeBlockIDRandom(), vote.StateID, vote.Height, commit, true},
		{"wrong height", chainID, vote.BlockID, vote.StateID, vote.Height - 1, commit, true},

		{"incorrect threshold block signature", chainID, vote.BlockID, vote.StateID, vote.Height,
			NewCommit(vote.Height, vote.Round, vote.BlockID, vote.StateID, quorumHash, nil, nil), true},

		{"incorrect threshold state signature", chainID, vote.BlockID, vote.StateID, vote.Height,
			NewCommit(vote.Height, vote.Round, vote.BlockID, vote.StateID,
				 quorumHash, vote.BlockSignature, nil), true},

		{"incorrect threshold block signature", chainID, vote.BlockID, vote.StateID, vote.Height,
			NewCommit(vote.Height, vote.Round, vote.BlockID, vote.StateID, quorumHash, vote2.BlockSignature, vote2.StateSignature), true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			err := vset.VerifyCommit(tc.chainID, tc.blockID, tc.stateID, tc.height, tc.commit)
			if tc.expErr {
				if assert.Error(t, err, "VerifyCommit") {
					assert.Contains(t, err.Error(), tc.description, "VerifyCommit")
				}
			} else {
				assert.NoError(t, err, "VerifyCommit")
			}
		})
	}
}

func TestValidatorSet_VerifyCommit_CheckThresholdSignatures(t *testing.T) {
	var (
		chainID = "test_chain_id"
		h       = int64(3)
		blockID = makeBlockIDRandom()
		stateID = makeStateIDRandom()
	)

	voteSet, valSet, vals := randVoteSet(h, 0, tmproto.PrecommitType, 4)
	commit, err := MakeCommit(blockID, stateID, h, 0, voteSet, vals)
	require.NoError(t, err)

	// malleate threshold sigs signature
	vote := voteSet.GetByIndex(3)
	v := vote.ToProto()
	err = vals[3].SignVote("CentaurusA", valSet.QuorumType, valSet.QuorumHash, v)
	require.NoError(t, err)
	commit.ThresholdBlockSignature = v.BlockSignature
	commit.ThresholdStateSignature = v.StateSignature

	err = valSet.VerifyCommit(chainID, blockID, stateID, h, commit)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "incorrect threshold block signature")
	}
}

func TestEmptySet(t *testing.T) {

	var valList []*Validator
	valSet := NewValidatorSet(valList, bls12381.PubKey{}, btcjson.LLMQType_5_60, crypto.QuorumHash{}, true)
	assert.Panics(t, func() { valSet.IncrementProposerPriority(1) })
	assert.Panics(t, func() { valSet.RescalePriorities(100) })
	assert.Panics(t, func() { valSet.shiftByAvgProposerPriority() })
	assert.Panics(t, func() { assert.Zero(t, computeMaxMinPriorityDiff(valSet)) })
	valSet.GetProposer()

	// Add to empty set
	proTxHashes := []crypto.ProTxHash{crypto.Sha256([]byte("v1")), crypto.Sha256([]byte("v2"))}
	valSetAdd, _ := GenerateTestValidatorSetWithProTxHashesDefaultPower(proTxHashes)
	assert.NoError(t, valSet.UpdateWithChangeSet(valSetAdd.Validators, valSetAdd.ThresholdPublicKey, crypto.RandQuorumHash()))
	verifyValidatorSet(t, valSet)

	// Delete all validators from set
	v1 := NewTestRemoveValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v1")))
	v2 := NewTestRemoveValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v1")))
	delList := []*Validator{v1, v2}
	assert.Error(t, valSet.UpdateWithChangeSet(delList, bls12381.PubKey{}, crypto.RandQuorumHash()))

	// Attempt delete from empty set
	assert.Error(t, valSet.UpdateWithChangeSet(delList, bls12381.PubKey{}, crypto.RandQuorumHash()))

}

func TestUpdatesForNewValidatorSet(t *testing.T) {

	addresses12 := []crypto.Address{crypto.Sha256([]byte("v1")), crypto.Sha256([]byte("v2"))}

	valSet, _ := GenerateTestValidatorSetWithProTxHashesDefaultPower(addresses12)
	verifyValidatorSet(t, valSet)

	// Verify duplicates are caught in NewValidatorSet() and it panics
	v111 := NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v1")))
	v112 := NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v1")))
	v113 := NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v1")))
	valList := []*Validator{v111, v112, v113}
	assert.Panics(t, func() { NewValidatorSet(valList, bls12381.PubKey{}, btcjson.LLMQType_5_60, crypto.QuorumHash{}, true) })

	// Verify set including validator with voting power 0 cannot be created
	v1 := NewTestRemoveValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v1")))
	v2 := NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v2")))
	v3 := NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v3")))
	valList = []*Validator{v1, v2, v3}
	assert.Panics(t, func() { NewValidatorSet(valList, bls12381.PubKey{}, btcjson.LLMQType_5_60, crypto.QuorumHash{}, true) })

	// Verify set including validator with negative voting power cannot be created
	v1 = NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v1")))
	v2 = &Validator{
		VotingPower:      -20,
		ProposerPriority: 0,
		ProTxHash:        crypto.Sha256([]byte("v2")),
	}
	v3 = NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte("v3")))
	valList = []*Validator{v1, v2, v3}
	assert.Panics(t, func() { NewValidatorSet(valList, bls12381.PubKey{}, btcjson.LLMQType_5_60, crypto.QuorumHash{}, true) })

}

type testVal struct {
	name  string
	power int64
}

type testProTxHashVal struct {
	proTxHash crypto.ProTxHash
	power int64
}

func permutation(valList []testVal) []testVal {
	if len(valList) == 0 {
		return nil
	}
	permList := make([]testVal, len(valList))
	perm := tmrand.Perm(len(valList))
	for i, v := range perm {
		permList[v] = valList[i]
	}
	return permList
}

func createNewValidatorList(testValList []testVal) []*Validator {
	valList := make([]*Validator, 0, len(testValList))
	for _, val := range testValList {
		valList = append(valList, NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte(val.name))))
	}
	sort.Sort(ValidatorsByProTxHashes(valList))
	return valList
}

func createNewValidatorSet(testValList []testVal) *ValidatorSet {
	proTxHashList := make([]crypto.ProTxHash, 0, len(testValList))
	powers := make([]int64, 0, len(testValList))
	for _, val := range testValList {
		proTxHashList = append(proTxHashList, crypto.Sha256([]byte(val.name)))
		powers = append(powers, val.power)
	}
	vals, _ := GenerateTestValidatorSetWithProTxHashes(proTxHashList, powers)
	return vals
}

func addValidatorsToValidatorSet(vals *ValidatorSet, testValList []testVal) ([]*Validator, crypto.PubKey) {
	addedProTxHashes := make([]ProTxHash, 0, len(testValList))
	removedProTxHashes := make([]ProTxHash, 0, len(testValList))
	removedVals := make([]*Validator, 0, len(testValList))
	combinedProTxHashes := make([]ProTxHash, 0, len(testValList)+len(vals.Validators))
	for _, val := range testValList {
		if val.power != 0 {
			valProTxHash := crypto.Sha256([]byte(val.name))
			_, value := vals.GetByProTxHash(valProTxHash)
			if value == nil {
				addedProTxHashes = append(addedProTxHashes, valProTxHash)
			}
		} else {
			valProTxHash := crypto.Sha256([]byte(val.name))
			_, value := vals.GetByProTxHash(valProTxHash)
			if value != nil {
				removedProTxHashes = append(removedProTxHashes, valProTxHash)
			}
			removedVals = append(removedVals, NewTestRemoveValidatorGeneratedFromProTxHash(crypto.Sha256([]byte(val.name))))
		}
	}
	originalProTxHashes := vals.GetProTxHashes()
	for _, oProTxHash := range originalProTxHashes {
		found := false
		for _, removedProTxHash := range removedProTxHashes {
			if bytes.Equal(oProTxHash.Bytes(), removedProTxHash.Bytes()) {
				found = true
			}
		}
		if !found {
			combinedProTxHashes = append(combinedProTxHashes, oProTxHash)
		}
	}
	combinedProTxHashes = append(combinedProTxHashes, addedProTxHashes...)
	if len(combinedProTxHashes) > 0 {
		rVals, _ := GenerateTestValidatorSetWithProTxHashesDefaultPower(combinedProTxHashes)
		rValidators := append(rVals.Validators, removedVals...)
		return rValidators, rVals.ThresholdPublicKey
	}
	return removedVals, nil

}

func valSetTotalProposerPriority(valSet *ValidatorSet) int64 {
	sum := int64(0)
	for _, val := range valSet.Validators {
		// mind overflow
		sum = safeAddClip(sum, val.ProposerPriority)
	}
	return sum
}

func verifyValidatorSet(t *testing.T, valSet *ValidatorSet) {
	// verify that the capacity and length of validators is the same
	assert.Equal(t, len(valSet.Validators), cap(valSet.Validators))

	// verify that the set's total voting power has been updated
	tvp := valSet.totalVotingPower
	valSet.updateTotalVotingPower()
	expectedTvp := valSet.TotalVotingPower()
	assert.Equal(t, expectedTvp, tvp,
		"expected TVP %d. Got %d, valSet=%s", expectedTvp, tvp, valSet)

	// verify that validator priorities are centered
	valsCount := int64(len(valSet.Validators))
	tpp := valSetTotalProposerPriority(valSet)
	assert.True(t, tpp < valsCount && tpp > -valsCount,
		"expected total priority in (-%d, %d). Got %d", valsCount, valsCount, tpp)

	// verify that priorities are scaled
	dist := computeMaxMinPriorityDiff(valSet)
	assert.True(t, dist <= PriorityWindowSizeFactor*tvp,
		"expected priority distance < %d. Got %d", PriorityWindowSizeFactor*tvp, dist)

	recoveredPublicKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(valSet.GetPublicKeys(), valSet.GetProTxHashesAsByteArrays())
	assert.NoError(t, err)
	assert.Equal(t, valSet.ThresholdPublicKey, recoveredPublicKey, "the validator set threshold public key must match the recovered public key")
}

func toTestProTxHashValList(valList []*Validator) []testProTxHashVal {
	testList := make([]testProTxHashVal, len(valList))
	for i, val := range valList {
		testList[i].proTxHash = val.ProTxHash
		testList[i].power = val.VotingPower
	}
	return testList
}

func switchToTestProTxHashValList(valList []testVal) []testProTxHashVal {
	testList := make([]testProTxHashVal, len(valList))
	for i, val := range valList {
		testList[i].proTxHash = crypto.Sha256([]byte(val.name))
		testList[i].power = val.power
	}
	return testList
}

func testValSet(nVals int) []testVal {
	vals := make([]testVal, nVals)
	for i := 0; i < nVals; i++ {
		vals[i] = testVal{fmt.Sprintf("v%d", i+1), DefaultDashVotingPower}
	}
	return vals
}

type valSetErrTestCase struct {
	startVals  []testVal
	updateVals []testVal
}

type valSetErrTestCaseWithErr struct {
	startVals  []testVal
	updateVals []testVal
	errString  string
}

func executeValSetErrTestCaseIgnoreThresholdPublicKey(t *testing.T, idx int, tt valSetErrTestCaseWithErr) {
	// create a new set and apply updates, keeping copies for the checks
	valSet := createNewValidatorSet(tt.startVals)
	valSetCopy := valSet.Copy()
	valList := createNewValidatorList(tt.updateVals)
	valListCopy := validatorListCopy(valList)
	err := valSet.UpdateWithChangeSet(valList, bls12381.GenPrivKey().PubKey(), crypto.RandQuorumHash())

	// for errors check the validator set has not been changed
	if assert.Error(t, err, "test %d", idx) {
		assert.Contains(t, err.Error(), tt.errString)
	}
	assert.Equal(t, valSet, valSetCopy, "test %v", idx)

	// check the parameter list has not changed
	assert.Equal(t, valList, valListCopy, "test %v", idx)
}

func executeValSetErrTestCase(t *testing.T, idx int, tt valSetErrTestCase) {
	// create a new set and apply updates, keeping copies for the checks
	valSet := createNewValidatorSet(tt.startVals)
	valSetCopy := valSet.Copy()
	valList, thresholdPublicKey := addValidatorsToValidatorSet(valSet, tt.updateVals)
	valListCopy := validatorListCopy(valList)
	err := valSet.UpdateWithChangeSet(valList, thresholdPublicKey, crypto.RandQuorumHash())

	// for errors check the validator set has not been changed
	assert.Error(t, err, "test %d", idx)
	assert.Equal(t, valSet, valSetCopy, "test %v", idx)

	// check the parameter list has not changed
	assert.Equal(t, valList, valListCopy, "test %v", idx)
}

func TestValSetUpdatesDuplicateEntries(t *testing.T) {
	testCases := []valSetErrTestCaseWithErr{
		// Duplicate entries in changes
		{ // first entry is duplicated change
			testValSet(2),
			[]testVal{{"v1", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			"duplicate entry Validator",
		},
		{ // second entry is duplicated change
			testValSet(2),
			[]testVal{{"v2", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
			"duplicate entry Validator",
		},
		{ // change duplicates are separated by a valid change
			testValSet(2),
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			"duplicate entry Validator",
		},
		{ // change duplicates are separated by a valid change
			testValSet(3),
			[]testVal{{"v1", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			"duplicate entry Validator",
		},

		// Duplicate entries in remove
		{ // first entry is duplicated remove
			testValSet(2),
			[]testVal{{"v1", 0}, {"v1", 0}},
			"duplicate entry Validator",
		},
		{ // second entry is duplicated remove
			testValSet(2),
			[]testVal{{"v2", 0}, {"v2", 0}},
			"duplicate entry Validator",
		},
		{ // remove duplicates are separated by a valid remove
			testValSet(2),
			[]testVal{{"v1", 0}, {"v2", 0}, {"v1", 0}},
			"duplicate entry Validator",
		},
		{ // remove duplicates are separated by a valid remove
			testValSet(3),
			[]testVal{{"v1", 0}, {"v3", 0}, {"v1", 0}},
			"duplicate entry Validator",
		},

		{ // remove and update same val
			testValSet(2),
			[]testVal{{"v1", 0}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			"duplicate entry Validator",
		},
		{ // duplicate entries in removes + changes
			testValSet(2),
			[]testVal{{"v1", 0}, {"v2", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", 0}},
			"duplicate entry Validator",
		},
		{ // duplicate entries in removes + changes
			testValSet(3),
			[]testVal{{"v1", 0}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", 0}},
			"duplicate entry Validator",
		},
	}

	for i, tt := range testCases {
		executeValSetErrTestCaseIgnoreThresholdPublicKey(t, i, tt)
	}
}

func TestValSetUpdatesOtherErrors(t *testing.T) {
	testCases := []valSetErrTestCase{
		{ // remove non-existing validator
			testValSet(2),
			[]testVal{{"v3", 0}},
		},
		{ // delete all validators
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}},
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
			testValSet(2),
			[]testVal{},
			testValSet(2),
		},
		{ // voting power changes
			testValSet(2),
			[]testVal{{"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
		},
		{ // add new validators
			[]testVal{{"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v4", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
		},
		{ // add new validator to middle
			[]testVal{{"v3", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v2", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
		},
		{ // add new validator to beginning
			[]testVal{{"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
		},
		{ // delete validators
			[]testVal{{"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v2", 0}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}},
		},
	}

	for i, tt := range valSetUpdatesBasicTests {
		// create a new set and apply updates, keeping copies for the checks
		valSet := createNewValidatorSet(tt.startVals)
		valList, thresholdPublicKey := addValidatorsToValidatorSet(valSet, tt.updateVals)
		err := valSet.UpdateWithChangeSet(valList, thresholdPublicKey, crypto.RandQuorumHash())
		assert.NoError(t, err, "test %d", i)

		valListCopy := validatorListCopy(valSet.Validators)
		// check that the voting power in the set's validators is not changing if the voting power
		// is changed in the list of validators previously passed as parameter to UpdateWithChangeSet.
		// this is to make sure copies of the validators are made by UpdateWithChangeSet.
		if len(valList) > 0 {
			valList[0].VotingPower++
			assert.Equal(t, toTestProTxHashValList(valListCopy), toTestProTxHashValList(valSet.Validators), "test %v", i)

		}

		// check the final validator list is as expected and the set is properly scaled and centered.
		assert.Equal(t, switchToTestProTxHashValList(tt.expectedVals), toTestProTxHashValList(valSet.Validators), "test %v", i)
		verifyValidatorSet(t, valSet)
	}
}

// Test that different permutations of an update give the same result.
func TestValSetUpdatesOrderIndependenceTestsExecute(t *testing.T) {
	// startVals - initial validators to create the set with
	// updateVals - a sequence of updates to be applied to the set.
	// updateVals is shuffled a number of times during testing to check for same resulting validator set.
	valSetUpdatesOrderTests := []struct {
		startVals  []testVal
		updateVals []testVal
	}{
		0: { // order of changes should not matter, the final validator sets should be the same
			[]testVal{{"v4", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v4", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}}},

		1: { // order of additions should not matter
			[]testVal{{"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v3", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}, {"v6", DefaultDashVotingPower}}},

		2: { // order of removals should not matter
			[]testVal{{"v4", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v1", 0}, {"v3", 0}, {"v4", 0}}},

		3: { // order of mixed operations should not matter
			[]testVal{{"v4", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			[]testVal{{"v1", 0}, {"v3", 0}, {"v2", 22}, {"v5", 50}, {"v4", 44}}},
	}

	for i, tt := range valSetUpdatesOrderTests {
		// create a new set and apply updates
		valSet := createNewValidatorSet(tt.startVals)
		valSetCopy := valSet.Copy()
		valList, thresholdPublicKey := addValidatorsToValidatorSet(valSet, tt.updateVals)
		assert.NoError(t, valSetCopy.UpdateWithChangeSet(valList, thresholdPublicKey, crypto.RandQuorumHash()))

		// save the result as expected for next updates
		valSetExp := valSetCopy.Copy()

		// perform at most 20 permutations on the updates and call UpdateWithChangeSet()
		n := len(tt.updateVals)
		maxNumPerms := tmmath.MinInt(20, n*n)
		for j := 0; j < maxNumPerms; j++ {
			// create a copy of original set and apply a random permutation of updates
			valSetCopy := valSet.Copy()
			valList, thresholdPublicKey := addValidatorsToValidatorSet(valSetCopy, permutation(tt.updateVals))

			// check there was no error and the set is properly scaled and centered.
			assert.NoError(t, valSetCopy.UpdateWithChangeSet(valList, thresholdPublicKey, crypto.RandQuorumHash()),
				"test %v failed for permutation %v", i, valList)
			verifyValidatorSet(t, valSetCopy)

			// verify the resulting test is same as the expected
			assert.Equal(t, valSetCopy.GetProTxHashes(), valSetExp.GetProTxHashes(),
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
			[]testVal{{"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}}},
		1: { // append
			[]testVal{{"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}},
			[]testVal{{"v6", DefaultDashVotingPower}},
			[]testVal{{"v6", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}}},
		2: { // insert
			[]testVal{{"v4", DefaultDashVotingPower}, {"v6", DefaultDashVotingPower}},
			[]testVal{{"v5", DefaultDashVotingPower}},
			[]testVal{{"v6", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}}},
		3: { // insert multi
			[]testVal{{"v4", DefaultDashVotingPower}, {"v6", DefaultDashVotingPower}, {"v9", DefaultDashVotingPower}},
			[]testVal{{"v5", DefaultDashVotingPower}, {"v7", DefaultDashVotingPower}, {"v8", DefaultDashVotingPower}},
			[]testVal{{"v8", DefaultDashVotingPower}, {"v7", DefaultDashVotingPower}, {"v6", DefaultDashVotingPower}, {"v9", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}}},
		// changes
		4: { // head
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}}},
		5: { // tail
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
			[]testVal{{"v2", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}}},
		6: { // middle
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}},
			[]testVal{{"v2", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}}},
		7: { // multi
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}}},
		// additions and changes
		8: {
			[]testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}},
			[]testVal{{"v1", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}}},
	}

	for i, tt := range valSetUpdatesBasicTests {
		// create a new validator set with the start values
		valSet := createNewValidatorSet(tt.startVals)
		// applyUpdates() with the update values
		valList := createNewValidatorList(tt.updateVals)

		valSet.applyUpdates(valList)

		// check the new list of validators for proper merge
		assert.Equal(t, toTestProTxHashValList(valSet.Validators), switchToTestProTxHashValList(tt.expectedVals), "test %v", i)
	}
}

type testVSetCfg struct {
	startVals    []testVal
	deletedVals  []testVal
	updatedVals  []testVal
	addedVals    []testVal
	expectedVals []testVal
}

func randTestVSetCfg(t *testing.T, nBase, nAddMax int) testVSetCfg {
	if nBase <= 0 || nAddMax < 0 {
		panic(fmt.Sprintf("bad parameters %v %v", nBase, nAddMax))
	}

	var nOld, nDel, nChanged, nAdd int

	nOld = int(tmrand.Uint()%uint(nBase)) + 1
	if nBase-nOld > 0 {
		nDel = int(tmrand.Uint() % uint(nBase-nOld))
	}
	nChanged = nBase - nOld - nDel

	if nAddMax > 0 {
		nAdd = tmrand.Int()%nAddMax + 1
	}

	cfg := testVSetCfg{}

	cfg.startVals = make([]testVal, nBase)
	cfg.deletedVals = make([]testVal, nDel)
	cfg.addedVals = make([]testVal, nAdd)
	cfg.updatedVals = make([]testVal, nChanged)
	cfg.expectedVals = make([]testVal, nBase-nDel+nAdd)

	for i := 0; i < nBase; i++ {
		cfg.startVals[i] = testVal{fmt.Sprintf("v%d", i), DefaultDashVotingPower}
		if i < nOld {
			cfg.expectedVals[i] = cfg.startVals[i]
		}
		if i >= nOld && i < nOld+nChanged {
			cfg.updatedVals[i-nOld] = testVal{fmt.Sprintf("v%d", i), DefaultDashVotingPower}
			cfg.expectedVals[i] = cfg.updatedVals[i-nOld]
		}
		if i >= nOld+nChanged {
			cfg.deletedVals[i-nOld-nChanged] = testVal{fmt.Sprintf("v%d", i), 0}
		}
	}

	for i := nBase; i < nBase+nAdd; i++ {
		cfg.addedVals[i-nBase] = testVal{fmt.Sprintf("v%d", i), DefaultDashVotingPower}
		cfg.expectedVals[i-nDel] = cfg.addedVals[i-nBase]
	}

	sort.Sort(testValsByVotingPower(cfg.startVals))
	sort.Sort(testValsByVotingPower(cfg.deletedVals))
	sort.Sort(testValsByVotingPower(cfg.updatedVals))
	sort.Sort(testValsByVotingPower(cfg.addedVals))
	sort.Sort(testValsByVotingPower(cfg.expectedVals))

	return cfg

}

func applyChangesToValSet(t *testing.T, expErr error, valSet *ValidatorSet, valsLists ...[]testVal) {
	changes := make([]testVal, 0)
	for _, valsList := range valsLists {
		changes = append(changes, valsList...)
	}
	valList, thresholdPublicKey := addValidatorsToValidatorSet(valSet, changes)
	err := valSet.UpdateWithChangeSet(valList, thresholdPublicKey, crypto.RandQuorumHash())
	if expErr != nil {
		assert.Equal(t, expErr, err)
	} else {
		assert.NoError(t, err)
	}
}

func TestValSetUpdatePriorityOrderTests(t *testing.T) {
	const nMaxElections int32 = 5000

	testCases := []testVSetCfg{
		0: { // remove high power validator, keep old equal lower power validators
			startVals:    []testVal{{"v3", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
			deletedVals:  []testVal{{"v3", 0}},
			updatedVals:  []testVal{},
			addedVals:    []testVal{},
			expectedVals: []testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
		},
		1: { // remove high power validator, keep old different power validators
			startVals:    []testVal{{"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			deletedVals:  []testVal{{"v3", 0}},
			updatedVals:  []testVal{},
			addedVals:    []testVal{},
			expectedVals: []testVal{{"v1", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
		},
		2: { // remove high power validator, add new low power validators, keep old lower power
			startVals:    []testVal{{"v3", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}, {"v1", DefaultDashVotingPower}},
			deletedVals:  []testVal{{"v3", 0}},
			updatedVals:  []testVal{{"v2", DefaultDashVotingPower}},
			addedVals:    []testVal{{"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}},
			expectedVals: []testVal{{"v1", DefaultDashVotingPower}, {"v4", DefaultDashVotingPower}, {"v5", DefaultDashVotingPower}, {"v2", DefaultDashVotingPower}},
		},

		// generate a configuration with 100 validators,
		// randomly select validators for updates and deletes, and
		// generate 10 new validators to be added
		3: randTestVSetCfg(t, 100, 10),
		//
		4: randTestVSetCfg(t, 1000, 100),
		//
		5: randTestVSetCfg(t, 10, 100),
		//
		6: randTestVSetCfg(t, 100, 1000),
		//
		7: randTestVSetCfg(t, 1000, 1000),
	}

	for i, cfg := range testCases {

		// create a new validator set
		valSet := createNewValidatorSet(cfg.startVals)
		verifyValidatorSet(t, valSet)

		// run election up to nMaxElections times, apply changes and verify that the priority order is correct
		verifyValSetUpdatePriorityOrder(t, valSet, cfg, nMaxElections, i)
	}
}

func verifyValSetUpdatePriorityOrder(t *testing.T, valSet *ValidatorSet, cfg testVSetCfg, nMaxElections int32, testNumber int) {
	// Run election up to nMaxElections times, sort validators by priorities
	valSet.IncrementProposerPriority(tmrand.Int31()%nMaxElections + 1)

	// apply the changes, get the updated validators, sort by priorities
	applyChangesToValSet(t, nil, valSet, cfg.addedVals, cfg.updatedVals, cfg.deletedVals)

	// basic checks
	testValSet := toTestProTxHashValList(valSet.Validators)
	assert.Equal(t, switchToTestProTxHashValList(cfg.expectedVals), testValSet, "(0) test number %d", testNumber)
	verifyValidatorSet(t, valSet)

	// verify that the added validators have the smallest priority:
	//  - they should be at the beginning of updatedValsPriSorted since it is
	//  sorted by priority
	if len(cfg.addedVals) > 0 {
		updatedValsPriSorted := validatorListCopy(valSet.Validators)
		sort.Sort(validatorsByPriority(updatedValsPriSorted))

		addedValsPriSlice := updatedValsPriSorted[:len(cfg.addedVals)]
		sort.Sort(ValidatorsByVotingPower(addedValsPriSlice))
		assert.Equal(t, switchToTestProTxHashValList(cfg.addedVals), toTestProTxHashValList(addedValsPriSlice), "(1) test number %d", testNumber)

		//  - and should all have the same priority
		expectedPri := addedValsPriSlice[0].ProposerPriority
		for _, val := range addedValsPriSlice[1:] {
			assert.Equal(t, expectedPri, val.ProposerPriority, "(2) test number %d", testNumber)
		}
	}
}

func TestNewValidatorSetFromExistingValidators(t *testing.T) {
	size := 5
	valSet, _ := GenerateValidatorSet(size)
	valSet.IncrementProposerPriority(3)

	newValSet0 := NewValidatorSet(valSet.Validators, valSet.ThresholdPublicKey, valSet.QuorumType, valSet.QuorumHash, true)
	assert.NotEqual(t, valSet, newValSet0)

	valSet.IncrementProposerPriority(2)
	newValSet1 := NewValidatorSet(valSet.Validators, valSet.ThresholdPublicKey, valSet.QuorumType, valSet.QuorumHash, true)
	assert.Equal(t, valSet, newValSet1)

	existingValSet, err := ValidatorSetFromExistingValidators(valSet.Validators, valSet.ThresholdPublicKey,
		valSet.QuorumType, valSet.QuorumHash)
	assert.NoError(t, err)
	assert.Equal(t, valSet, existingValSet)
	assert.Equal(t, valSet.CopyIncrementProposerPriority(3), existingValSet.CopyIncrementProposerPriority(3))
}

func TestSafeMul(t *testing.T) {
	testCases := []struct {
		a        int64
		b        int64
		c        int64
		overflow bool
	}{
		0: {0, 0, 0, false},
		1: {1, 0, 0, false},
		2: {2, 3, 6, false},
		3: {2, -3, -6, false},
		4: {-2, -3, 6, false},
		5: {-2, 3, -6, false},
		6: {math.MaxInt64, 1, math.MaxInt64, false},
		7: {math.MaxInt64 / 2, 2, math.MaxInt64 - 1, false},
		8: {math.MaxInt64 / 2, 3, 0, true},
		9: {math.MaxInt64, 2, 0, true},
	}

	for i, tc := range testCases {
		c, overflow := safeMul(tc.a, tc.b)
		assert.Equal(t, tc.c, c, "#%d", i)
		assert.Equal(t, tc.overflow, overflow, "#%d", i)
	}
}

func TestValidatorSetProtoBuf(t *testing.T) {
	valset, _ := GenerateValidatorSet(10)
	valset2, _ := GenerateValidatorSet(10)
	valset2.Validators[0] = &Validator{}

	valset3, _ := GenerateValidatorSet(10)
	valset3.Proposer = nil

	valset4, _ := GenerateValidatorSet(10)
	valset4.Proposer = &Validator{}

	testCases := []struct {
		msg      string
		v1       *ValidatorSet
		expPass1 bool
		expPass2 bool
	}{
		{"success", valset, true, true},
		{"fail valSet2, pubkey empty", valset2, false, false},
		{"fail nil Proposer", valset3, false, false},
		{"fail empty Proposer", valset4, false, false},
		{"fail empty valSet", &ValidatorSet{}, true, false},
		{"false nil", nil, true, false},
	}
	for _, tc := range testCases {
		protoValSet, err := tc.v1.ToProto()
		if tc.expPass1 {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		valSet, err := ValidatorSetFromProto(protoValSet)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.EqualValues(t, tc.v1, valSet, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

//---------------------
// Sort validators by priority and address
type validatorsByPriority []*Validator

func (valz validatorsByPriority) Len() int {
	return len(valz)
}

func (valz validatorsByPriority) Less(i, j int) bool {
	if valz[i].ProposerPriority < valz[j].ProposerPriority {
		return true
	}
	if valz[i].ProposerPriority > valz[j].ProposerPriority {
		return false
	}
	return bytes.Compare(valz[i].ProTxHash, valz[j].ProTxHash) < 0
}

func (valz validatorsByPriority) Swap(i, j int) {
	valz[i], valz[j] = valz[j], valz[i]
}

//-------------------------------------

type testValsByVotingPower []testVal

func (tvals testValsByVotingPower) Len() int {
	return len(tvals)
}

// Here we need to sort by the pro_tx_hash and not the name if the power is equal, in the test the pro_tx_hash is derived
//  from the name by applying a single SHA256
func (tvals testValsByVotingPower) Less(i, j int) bool {
	if tvals[i].power == tvals[j].power {
		return bytes.Compare(crypto.Sha256([]byte(tvals[i].name)), crypto.Sha256([]byte(tvals[j].name))) == -1
	}
	return tvals[i].power > tvals[j].power
}

func (tvals testValsByVotingPower) Swap(i, j int) {
	tvals[i], tvals[j] = tvals[j], tvals[i]
}

//-------------------------------------
// Benchmark tests
//
func BenchmarkUpdates(b *testing.B) {
	const (
		n = 100
		m = 2000
	)
	// Init with n validators
	proTxHashes0 := make([]crypto.ProTxHash, n)
	for j := 0; j < n; j++ {
		proTxHashes0[j] = crypto.Sha256([]byte(fmt.Sprintf("v%d", j)))
	}
	valSet, _ := GenerateTestValidatorSetWithProTxHashesDefaultPower(proTxHashes0)

	proTxHashes1 := make([]crypto.ProTxHash, n+m)
	newValList := make([]*Validator, m)
	for j := 0; j < n+m; j++ {
		proTxHashes1[j] = []byte(fmt.Sprintf("v%d", j))
		if j >= n {
			newValList[j-n] = NewTestValidatorGeneratedFromProTxHash(crypto.Sha256([]byte(fmt.Sprintf("v%d", j))))
		}
	}
	valSet2, _ := GenerateTestValidatorSetWithProTxHashesDefaultPower(proTxHashes1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Add m validators to valSetCopy
		valSetCopy := valSet.Copy()
		assert.NoError(b, valSetCopy.UpdateWithChangeSet(newValList, valSet2.ThresholdPublicKey, crypto.RandQuorumHash()))
	}
}
