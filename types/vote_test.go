package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

func examplePrevote() *Vote {
	return exampleVote(byte(PrevoteType))
}

func examplePrecommit() *Vote {
	return exampleVote(byte(PrecommitType))
}

func exampleVote(t byte) *Vote {
	var stamp, err = time.Parse(TimeFormat, "2017-12-25T03:00:01.234Z")
	if err != nil {
		panic(err)
	}

	return &Vote{
		Type:      SignedMsgType(t),
		Height:    12345,
		Round:     2,
		Timestamp: stamp,
		BlockID: BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartsHeader: PartSetHeader{
				Total: 1000000,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		ValidatorIndex:   56789,
	}
}

// Ensure that Vote and CommitSig have the same encoding.
// This ensures using CommitSig isn't a breaking change.
// This test will fail and can be removed once CommitSig contains only sigs and
// timestamps.
func TestVoteEncoding(t *testing.T) {
	vote := examplePrecommit()
	commitSig := vote.CommitSig()
	cdc := amino.NewCodec()
	bz1 := cdc.MustMarshalBinaryBare(vote)
	bz2 := cdc.MustMarshalBinaryBare(commitSig)
	assert.Equal(t, bz1, bz2)
}

func TestVoteSignable(t *testing.T) {
	vote := examplePrecommit()
	signBytes := vote.SignBytes("test_chain_id")

	expected, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeVote("test_chain_id", vote))
	require.NoError(t, err)

	require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Vote.")
}

func TestVoteSignBytesTestVectors(t *testing.T) {

	tests := []struct {
		chainID string
		vote    *Vote
		want    []byte
	}{
		0: {
			"", &Vote{},
			// NOTE: Height and Round are skipped here. This case needs to be considered while parsing.
			[]byte{0xd, 0x2a, 0xb, 0x8, 0x80, 0x92, 0xb8, 0xc3, 0x98, 0xfe, 0xff, 0xff, 0xff, 0x1},
		},
		// with proper (fixed size) height and round (PreCommit):
		1: {
			"", &Vote{Height: 1, Round: 1, Type: PrecommitType},
			[]byte{
				0x21,                                   // length
				0x8,                                    // (field_number << 3) | wire_type
				0x2,                                    // PrecommitType
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				0x2a, // (field_number << 3) | wire_type
				// remaining fields (timestamp):
				0xb, 0x8, 0x80, 0x92, 0xb8, 0xc3, 0x98, 0xfe, 0xff, 0xff, 0xff, 0x1},
		},
		// with proper (fixed size) height and round (PreVote):
		2: {
			"", &Vote{Height: 1, Round: 1, Type: PrevoteType},
			[]byte{
				0x21,                                   // length
				0x8,                                    // (field_number << 3) | wire_type
				0x1,                                    // PrevoteType
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				0x2a, // (field_number << 3) | wire_type
				// remaining fields (timestamp):
				0xb, 0x8, 0x80, 0x92, 0xb8, 0xc3, 0x98, 0xfe, 0xff, 0xff, 0xff, 0x1},
		},
		3: {
			"", &Vote{Height: 1, Round: 1},
			[]byte{
				0x1f,                                   // length
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaining fields (timestamp):
				0x2a,
				0xb, 0x8, 0x80, 0x92, 0xb8, 0xc3, 0x98, 0xfe, 0xff, 0xff, 0xff, 0x1},
		},
		// containing non-empty chain_id:
		4: {
			"test_chain_id", &Vote{Height: 1, Round: 1},
			[]byte{
				0x2e,                                   // length
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaining fields:
				0x2a,                                                                // (field_number << 3) | wire_type
				0xb, 0x8, 0x80, 0x92, 0xb8, 0xc3, 0x98, 0xfe, 0xff, 0xff, 0xff, 0x1, // timestamp
				0x32,                                                                               // (field_number << 3) | wire_type
				0xd, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x5f, 0x69, 0x64}, // chainID
		},
	}
	for i, tc := range tests {
		got := tc.vote.SignBytes(tc.chainID)
		require.Equal(t, tc.want, got, "test case #%v: got unexpected sign bytes for Vote.", i)
	}
}

func TestVoteProposalNotEq(t *testing.T) {
	cv := CanonicalizeVote("", &Vote{Height: 1, Round: 1})
	p := CanonicalizeProposal("", &Proposal{Height: 1, Round: 1})
	vb, err := cdc.MarshalBinaryLengthPrefixed(cv)
	require.NoError(t, err)
	pb, err := cdc.MarshalBinaryLengthPrefixed(p)
	require.NoError(t, err)
	require.NotEqual(t, vb, pb)
}

func TestVoteVerifySignature(t *testing.T) {
	privVal := NewMockPV()
	pubkey := privVal.GetPubKey()

	vote := examplePrecommit()
	signBytes := vote.SignBytes("test_chain_id")

	// sign it
	err := privVal.SignVote("test_chain_id", vote)
	require.NoError(t, err)

	// verify the same vote
	valid := pubkey.VerifyBytes(vote.SignBytes("test_chain_id"), vote.Signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	precommit := new(Vote)
	bs, err := cdc.MarshalBinaryLengthPrefixed(vote)
	require.NoError(t, err)
	err = cdc.UnmarshalBinaryLengthPrefixed(bs, &precommit)
	require.NoError(t, err)

	// verify the transmitted vote
	newSignBytes := precommit.SignBytes("test_chain_id")
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubkey.VerifyBytes(newSignBytes, precommit.Signature)
	require.True(t, valid)
}

func TestIsVoteTypeValid(t *testing.T) {
	tc := []struct {
		name string
		in   SignedMsgType
		out  bool
	}{
		{"Prevote", PrevoteType, true},
		{"Precommit", PrecommitType, true},
		{"InvalidType", SignedMsgType(0x3), false},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(st *testing.T) {
			if rs := IsVoteTypeValid(tt.in); rs != tt.out {
				t.Errorf("Got unexpected Vote type. Expected:\n%v\nGot:\n%v", rs, tt.out)
			}
		})
	}
}

func TestVoteVerify(t *testing.T) {
	privVal := NewMockPV()
	pubkey := privVal.GetPubKey()

	vote := examplePrevote()
	vote.ValidatorAddress = pubkey.Address()

	err := vote.Verify("test_chain_id", ed25519.GenPrivKey().PubKey())
	if assert.Error(t, err) {
		assert.Equal(t, ErrVoteInvalidValidatorAddress, err)
	}

	err = vote.Verify("test_chain_id", pubkey)
	if assert.Error(t, err) {
		assert.Equal(t, ErrVoteInvalidSignature, err)
	}
}

func TestMaxVoteBytes(t *testing.T) {
	// time is varint encoded so need to pick the max.
	// year int, month Month, day, hour, min, sec, nsec int, loc *Location
	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)

	vote := &Vote{
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		ValidatorIndex:   math.MaxInt64,
		Height:           math.MaxInt64,
		Round:            math.MaxInt64,
		Timestamp:        timestamp,
		Type:             PrevoteType,
		BlockID: BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartsHeader: PartSetHeader{
				Total: math.MaxInt64,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
	}

	privVal := NewMockPV()
	err := privVal.SignVote("test_chain_id", vote)
	require.NoError(t, err)

	bz, err := cdc.MarshalBinaryLengthPrefixed(vote)
	require.NoError(t, err)

	assert.EqualValues(t, MaxVoteBytes, len(bz))
}

func TestVoteString(t *testing.T) {
	str := examplePrecommit().String()
	expected := `Vote{56789:6AF1F4111082 12345/02/2(Precommit) 8B01023386C3 000000000000 @ 2017-12-25T03:00:01.234Z}`
	if str != expected {
		t.Errorf("Got unexpected string for Vote. Expected:\n%v\nGot:\n%v", expected, str)
	}

	str2 := examplePrevote().String()
	expected = `Vote{56789:6AF1F4111082 12345/02/1(Prevote) 8B01023386C3 000000000000 @ 2017-12-25T03:00:01.234Z}`
	if str2 != expected {
		t.Errorf("Got unexpected string for Vote. Expected:\n%v\nGot:\n%v", expected, str2)
	}
}

func TestVoteValidateBasic(t *testing.T) {
	privVal := NewMockPV()

	testCases := []struct {
		testName     string
		malleateVote func(*Vote)
		expectErr    bool
	}{
		{"Good Vote", func(v *Vote) {}, false},
		{"Negative Height", func(v *Vote) { v.Height = -1 }, true},
		{"Negative Round", func(v *Vote) { v.Round = -1 }, true},
		{"Invalid BlockID", func(v *Vote) { v.BlockID = BlockID{[]byte{1, 2, 3}, PartSetHeader{111, []byte("blockparts")}} }, true},
		{"Invalid Address", func(v *Vote) { v.ValidatorAddress = make([]byte, 1) }, true},
		{"Invalid ValidatorIndex", func(v *Vote) { v.ValidatorIndex = -1 }, true},
		{"Invalid Signature", func(v *Vote) { v.Signature = nil }, true},
		{"Too big Signature", func(v *Vote) { v.Signature = make([]byte, MaxSignatureSize+1) }, true},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			vote := examplePrecommit()
			err := privVal.SignVote("test_chain_id", vote)
			require.NoError(t, err)
			tc.malleateVote(vote)
			assert.Equal(t, tc.expectErr, vote.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
