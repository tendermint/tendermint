package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func examplePrevote() *Vote {
	return exampleVote(VoteTypePrevote)
}

func examplePrecommit() *Vote {
	return exampleVote(VoteTypePrecommit)
}

func exampleVote(t byte) *Vote {
	var stamp, err = time.Parse(TimeFormat, "2017-12-25T03:00:01.234Z")
	if err != nil {
		panic(err)
	}

	return &Vote{
		ValidatorAddress: tmhash.Sum([]byte("validator_address")),
		ValidatorIndex:   56789,
		Height:           12345,
		Round:            2,
		Timestamp:        stamp,
		Type:             t,
		BlockID: BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartsHeader: PartSetHeader{
				Total: 1000000,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
	}
}

func TestVoteSignable(t *testing.T) {
	vote := examplePrecommit()
	signBytes := vote.SignBytes("test_chain_id")

	expected, err := cdc.MarshalBinary(CanonicalizeVote("test_chain_id", vote))
	require.NoError(t, err)

	require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Vote.")
}

func TestVoteSignableTestVectors(t *testing.T) {
	voteWithVersion := CanonicalizeVote("", &Vote{Height: 1, Round: 1})
	voteWithVersion.Version = 123

	tests := []struct {
		canonicalVote CanonicalVote
		want          []byte
	}{
		{
			CanonicalizeVote("", &Vote{}),
			// XXX: Here Height and Round are skipped. This probably will be cumbersome to to parse in the HSM:
			[]byte{0xb, 0x2a, 0x9, 0x9, 0x0, 0x9, 0x6e, 0x88, 0xf1, 0xff, 0xff, 0xff},
		},
		// with proper (fixed size) height and round:
		{
			CanonicalizeVote("", &Vote{Height: 1, Round: 1}),
			[]byte{
				0x1d,                                   // total length
				0x11,                                   // (field_number << 3) | wire_type (version is missing)
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaining fields (timestamp):
				0x2a, // (field_number << 3) | wire_type
				0x9, 0x9, 0x0, 0x9, 0x6e, 0x88, 0xf1, 0xff, 0xff, 0xff},
		},
		// containing version:
		{
			voteWithVersion,
			[]byte{
				0x26,                                    // total length
				0x9,                                     // (field_number << 3) | wire_type
				0x7b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // version (123)
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaining fields (timestamp):
				0x2a, 0x9, 0x9, 0x0, 0x9, 0x6e, 0x88, 0xf1, 0xff, 0xff, 0xff},
		},
		// containing non-empty chain_id:
		{
			CanonicalizeVote("test_chain_id", &Vote{Height: 1, Round: 1}),
			[]byte{
				0x2c,                                   // total length
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaining fields:
				0x2a,                                                   // (field_number << 3) | wire_type (here: 5 << 3 | 2)
				0x9, 0x9, 0x0, 0x9, 0x6e, 0x88, 0xf1, 0xff, 0xff, 0xff, // timestamp
				0x3a,                                                                               // (field_number << 3) | wire_type
				0xd, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x5f, 0x69, 0x64}, // chainID
		},
	}
	for i, tc := range tests {
		got, err := cdc.MarshalBinary(tc.canonicalVote)
		require.NoError(t, err)

		require.Equal(t, tc.want, got, "test case #%v: got unexpected sign bytes for Vote.", i)
	}
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
	bs, err := cdc.MarshalBinary(vote)
	require.NoError(t, err)
	err = cdc.UnmarshalBinary(bs, &precommit)
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
		in   byte
		out  bool
	}{
		{"Prevote", VoteTypePrevote, true},
		{"Precommit", VoteTypePrecommit, true},
		{"InvalidType", byte(3), false},
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
	vote := &Vote{
		ValidatorAddress: tmhash.Sum([]byte("validator_address")),
		ValidatorIndex:   math.MaxInt64,
		Height:           math.MaxInt64,
		Round:            math.MaxInt64,
		Timestamp:        tmtime.Now(),
		Type:             VoteTypePrevote,
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

	bz, err := cdc.MarshalBinary(vote)
	require.NoError(t, err)

	assert.EqualValues(t, MaxVoteBytes, len(bz))
}
