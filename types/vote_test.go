package types

import (
	"context"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func examplePrevote(t *testing.T) *Vote {
	t.Helper()
	return exampleVote(t, byte(tmproto.PrevoteType))
}

func examplePrecommit(t testing.TB) *Vote {
	t.Helper()
	return exampleVote(t, byte(tmproto.PrecommitType))
}

func exampleVote(tb testing.TB, t byte) *Vote {
	tb.Helper()
	var stamp, err = time.Parse(TimeFormat, "2017-12-25T03:00:01.234Z")
	require.NoError(tb, err)

	return &Vote{
		Type:      tmproto.SignedMsgType(t),
		Height:    12345,
		Round:     2,
		Timestamp: stamp,
		BlockID: BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartSetHeader: PartSetHeader{
				Total: 1000000,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		ValidatorIndex:   56789,
	}
}
func TestVoteSignable(t *testing.T) {
	vote := examplePrecommit(t)
	v := vote.ToProto()
	signBytes := VoteSignBytes("test_chain_id", v)
	pb := CanonicalizeVote("test_chain_id", v)
	expected, err := protoio.MarshalDelimited(&pb)
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
			"", &Vote{Height: 1, Round: 1, Type: tmproto.PrecommitType},
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
			"", &Vote{Height: 1, Round: 1, Type: tmproto.PrevoteType},
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
				// (field_number << 3) | wire_type
				0x32,
				0xd, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x5f, 0x69, 0x64}, // chainID
		},
		// containing vote extension
		5: {
			"test_chain_id", &Vote{Height: 1, Round: 1, VoteExtension: VoteExtension{
				AppDataToSign:             []byte("signed"),
				AppDataSelfAuthenticating: []byte("auth"),
			}},
			[]byte{
				0x38,                                   // length
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaning fields:
				0x2a,                                                                // (field_number << 3) | wire_type
				0xb, 0x8, 0x80, 0x92, 0xb8, 0xc3, 0x98, 0xfe, 0xff, 0xff, 0xff, 0x1, // timestamp
				// (field_number << 3) | wire_type
				0x32,
				0xd, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x5f, 0x69, 0x64, // chainID
				// (field_number << 3) | wire_type
				0x3a,
				0x8,                                // length
				0xa,                                // (field_number << 3) | wire_type
				0x6,                                // length
				0x73, 0x69, 0x67, 0x6e, 0x65, 0x64, // AppDataSigned
				// SelfAuthenticating data is excluded on signing
			}, // chainID
		},
	}
	for i, tc := range tests {
		v := tc.vote.ToProto()
		got := VoteSignBytes(tc.chainID, v)
		assert.Equal(t, len(tc.want), len(got), "test case #%v: got unexpected sign bytes length for Vote.", i)
		assert.Equal(t, tc.want, got, "test case #%v: got unexpected sign bytes for Vote.", i)
	}
}

func TestVoteProposalNotEq(t *testing.T) {
	cv := CanonicalizeVote("", &tmproto.Vote{Height: 1, Round: 1})
	p := CanonicalizeProposal("", &tmproto.Proposal{Height: 1, Round: 1})
	vb, err := proto.Marshal(&cv)
	require.NoError(t, err)
	pb, err := proto.Marshal(&p)
	require.NoError(t, err)
	require.NotEqual(t, vb, pb)
}

func TestVoteVerifySignature(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	privVal := NewMockPV()
	pubkey, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)

	vote := examplePrecommit(t)
	v := vote.ToProto()
	signBytes := VoteSignBytes("test_chain_id", v)

	// sign it
	err = privVal.SignVote(ctx, "test_chain_id", v)
	require.NoError(t, err)

	// verify the same vote
	valid := pubkey.VerifySignature(VoteSignBytes("test_chain_id", v), v.Signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	precommit := new(tmproto.Vote)
	bs, err := proto.Marshal(v)
	require.NoError(t, err)
	err = proto.Unmarshal(bs, precommit)
	require.NoError(t, err)

	// verify the transmitted vote
	newSignBytes := VoteSignBytes("test_chain_id", precommit)
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubkey.VerifySignature(newSignBytes, precommit.Signature)
	require.True(t, valid)
}

func TestIsVoteTypeValid(t *testing.T) {
	tc := []struct {
		name string
		in   tmproto.SignedMsgType
		out  bool
	}{
		{"Prevote", tmproto.PrevoteType, true},
		{"Precommit", tmproto.PrecommitType, true},
		{"InvalidType", tmproto.SignedMsgType(0x3), false},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(st *testing.T) {
			if rs := IsVoteTypeValid(tt.in); rs != tt.out {
				t.Errorf("got unexpected Vote type. Expected:\n%v\nGot:\n%v", rs, tt.out)
			}
		})
	}
}

func TestVoteVerify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	privVal := NewMockPV()
	pubkey, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)

	vote := examplePrevote(t)
	vote.ValidatorAddress = pubkey.Address()

	err = vote.Verify("test_chain_id", ed25519.GenPrivKey().PubKey())
	if assert.Error(t, err) {
		assert.Equal(t, ErrVoteInvalidValidatorAddress, err)
	}

	err = vote.Verify("test_chain_id", pubkey)
	if assert.Error(t, err) {
		assert.Equal(t, ErrVoteInvalidSignature, err)
	}
}

func TestVoteString(t *testing.T) {
	str := examplePrecommit(t).String()
	expected := `Vote{56789:6AF1F4111082 12345/02/SIGNED_MSG_TYPE_PRECOMMIT(Precommit) 8B01023386C3 000000000000 000000000000 @ 2017-12-25T03:00:01.234Z}` //nolint:lll //ignore line length for tests
	if str != expected {
		t.Errorf("got unexpected string for Vote. Expected:\n%v\nGot:\n%v", expected, str)
	}

	str2 := examplePrevote(t).String()
	expected = `Vote{56789:6AF1F4111082 12345/02/SIGNED_MSG_TYPE_PREVOTE(Prevote) 8B01023386C3 000000000000 000000000000 @ 2017-12-25T03:00:01.234Z}` //nolint:lll //ignore line length for tests
	if str2 != expected {
		t.Errorf("got unexpected string for Vote. Expected:\n%v\nGot:\n%v", expected, str2)
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
		{"Invalid BlockID", func(v *Vote) {
			v.BlockID = BlockID{[]byte{1, 2, 3}, PartSetHeader{111, []byte("blockparts")}}
		}, true},
		{"Invalid Address", func(v *Vote) { v.ValidatorAddress = make([]byte, 1) }, true},
		{"Invalid ValidatorIndex", func(v *Vote) { v.ValidatorIndex = -1 }, true},
		{"Invalid Signature", func(v *Vote) { v.Signature = nil }, true},
		{"Too big Signature", func(v *Vote) { v.Signature = make([]byte, MaxSignatureSize+1) }, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			vote := examplePrecommit(t)
			v := vote.ToProto()
			err := privVal.SignVote(ctx, "test_chain_id", v)
			vote.Signature = v.Signature
			require.NoError(t, err)
			tc.malleateVote(vote)
			assert.Equal(t, tc.expectErr, vote.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestVoteProtobuf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	privVal := NewMockPV()
	vote := examplePrecommit(t)
	v := vote.ToProto()
	err := privVal.SignVote(ctx, "test_chain_id", v)
	vote.Signature = v.Signature
	require.NoError(t, err)

	testCases := []struct {
		msg     string
		v1      *Vote
		expPass bool
	}{
		{"success", vote, true},
		{"fail vote validate basic", &Vote{}, false},
		{"failure nil", nil, false},
	}
	for _, tc := range testCases {
		protoProposal := tc.v1.ToProto()

		v, err := VoteFromProto(protoProposal)
		if tc.expPass {
			require.NoError(t, err)
			require.Equal(t, tc.v1, v, tc.msg)
		} else {
			require.Error(t, err)
		}
	}
}

var sink interface{}

func getSampleCommit(ctx context.Context, t testing.TB) *Commit {
	t.Helper()

	lastID := makeBlockIDRandom()
	voteSet, _, vals := randVoteSet(ctx, t, 2, 1, tmproto.PrecommitType, 10, 1)
	commit, err := makeCommit(ctx, lastID, 2, 1, voteSet, vals, time.Now())

	require.NoError(t, err)

	return commit
}

func BenchmarkVoteSignBytes(b *testing.B) {
	protoVote := examplePrecommit(b).ToProto()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = VoteSignBytes("test_chain_id", protoVote)
	}

	if sink == nil {
		b.Fatal("Benchmark did not run")
	}

	// Reset the sink.
	sink = (interface{})(nil)
}

func BenchmarkCommitVoteSignBytes(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sampleCommit := getSampleCommit(ctx, b)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for index := range sampleCommit.Signatures {
			sink = sampleCommit.VoteSignBytes("test_chain_id", int32(index))
		}
	}

	if sink == nil {
		b.Fatal("Benchmark did not run")
	}

	// Reset the sink.
	sink = (interface{})(nil)
}
