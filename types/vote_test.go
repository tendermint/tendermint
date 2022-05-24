package types

import (
	"context"
	"strings"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func examplePrevote(t *testing.T) *Vote {
	t.Helper()
	return exampleVote(t, byte(tmproto.PrevoteType))
}

func examplePrecommit(t testing.TB) *Vote {
	t.Helper()
	vote := exampleVote(t, byte(tmproto.PrecommitType))
	vote.ExtensionSignature = []byte("signature")
	return vote
}

func exampleVote(tb testing.TB, t byte) *Vote {
	tb.Helper()

	return &Vote{
		Type:   tmproto.SignedMsgType(t),
		Height: 12345,
		Round:  2,
		BlockID: BlockID{
			Hash: crypto.Checksum([]byte("blockID_hash")),
			PartSetHeader: PartSetHeader{
				Total: 1000000,
				Hash:  crypto.Checksum([]byte("blockID_part_set_header_hash")),
			},
		},
		ValidatorProTxHash: crypto.ProTxHashFromSeedBytes([]byte("validator_pro_tx_hash")),
		ValidatorIndex:     56789,
	}
}

func TestVoteSignable(t *testing.T) {
	vote := examplePrecommit(t)
	v := vote.ToProto()
	signBytes := VoteBlockSignBytes("test_chain_id", v)
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
			[]byte{0x0},
		},
		// with proper (fixed size) height and round (PreCommit):
		1: {
			"", &Vote{Height: 1, Round: 1, Type: tmproto.PrecommitType},
			[]byte{
				0x14,                                   // length
				0x8,                                    // (field_number << 3) | wire_type
				0x2,                                    // PrecommitType
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
			},
		},
		// with proper (fixed size) height and round (PreVote):
		2: {
			"", &Vote{Height: 1, Round: 1, Type: tmproto.PrevoteType},
			[]byte{
				0x14,                                   // length
				0x8,                                    // (field_number << 3) | wire_type
				0x1,                                    // PrevoteType
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
			},
		},
		3: {
			"", &Vote{Height: 1, Round: 1},
			[]byte{
				0x12,                                   // length
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
			},
		},
		// containing non-empty chain_id:
		4: {
			"test_chain_id", &Vote{Height: 1, Round: 1},
			[]byte{
				0x21,                                   // length
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaining fields:
				// (field_number << 3) | wire_type
				0x32,
				0xd, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x5f, 0x69, 0x64}, // chainID
		},
		// containing vote extension
		5: {
			"test_chain_id", &Vote{
				Height:    1,
				Round:     1,
				Extension: []byte("extension"),
			},
			[]byte{
				0x21,                                   // length
				0x11,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // height
				0x19,                                   // (field_number << 3) | wire_type
				0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // round
				// remaning fields:
				// (field_number << 3) | wire_type
				0x32,
				0xd, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x5f, 0x69, 0x64, // chainID
			}, // chainID
		},
	}
	for i, tc := range tests {
		v := tc.vote.ToProto()
		got := VoteBlockSignBytes(tc.chainID, v)
		assert.Equal(t, len(tc.want), len(got), "test case #%v: got unexpected sign bytes length for Vote.", i)
		assert.Equal(t, tc.want, got, "test case #%v: got unexpected sign bytes for Vote.", i)
	}
}

func TestVoteStateSignBytesTestVectors(t *testing.T) {
	tests := []struct {
		chainID string
		height  int64
		apphash []byte
		want    []byte
	}{
		0: {
			"", 1, []byte("12345678901234567890123456789012"),
			// NOTE: Height and Round are skipped here. This case needs to be considered while parsing.
			[]byte{0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38,
				0x39, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x30, 0x31, 0x32, 0x33,
				0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x30, 0x31, 0x32},
		},
	}
	for i, tc := range tests {
		sid := StateID{
			Height:      tc.height,
			LastAppHash: tc.apphash,
		}
		got := sid.SignBytes(tc.chainID)
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

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)
	pubkey, err := privVal.GetPubKey(context.Background(), quorumHash)
	require.NoError(t, err)

	vote := examplePrecommit(t)
	v := vote.ToProto()
	stateID := RandStateID().WithHeight(vote.Height - 1)
	quorumType := btcjson.LLMQType_5_60
	signID := VoteBlockSignID("test_chain_id", v, quorumType, quorumHash)
	signStateID := stateID.SignID("test_chain_id", quorumType, quorumHash)

	// sign it
	err = privVal.SignVote(ctx, "test_chain_id", quorumType, quorumHash, v, stateID, nil)
	require.NoError(t, err)

	// verify the same vote
	valid := pubkey.VerifySignatureDigest(signID, v.BlockSignature)
	require.True(t, valid)

	// verify the same vote
	valid = pubkey.VerifySignatureDigest(signStateID, v.StateSignature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	precommit := new(tmproto.Vote)
	bs, err := proto.Marshal(v)
	require.NoError(t, err)
	err = proto.Unmarshal(bs, precommit)
	require.NoError(t, err)

	// verify the transmitted vote
	newSignID := VoteBlockSignID("test_chain_id", precommit, quorumType, quorumHash)
	newSignStateID := stateID.SignID("test_chain_id", quorumType, quorumHash)
	require.Equal(t, string(signID), string(newSignID))
	require.Equal(t, string(signStateID), string(newSignStateID))
	valid = pubkey.VerifySignatureDigest(newSignID, precommit.BlockSignature)
	require.True(t, valid)
	valid = pubkey.VerifySignatureDigest(newSignStateID, precommit.StateSignature)
	require.True(t, valid)
}

// TestVoteExtension tests that the vote verification behaves correctly in each case
// of vote extension being set on the vote.
func TestVoteExtension(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCases := []struct {
		name             string
		extension        []byte
		includeSignature bool
		expectError      bool
	}{
		{
			name:             "all fields present",
			extension:        []byte("extension"),
			includeSignature: true,
			expectError:      false,
		},
		// TODO(thane): Re-enable once
		// https://github.com/tendermint/tendermint/issues/8272 is resolved
		//{
		//	name:             "no extension signature",
		//	extension:        []byte("extension"),
		//	includeSignature: false,
		//	expectError:      true,
		//},
		{
			name:             "empty extension",
			includeSignature: true,
			expectError:      false,
		},
		// TODO: Re-enable once
		// https://github.com/tendermint/tendermint/issues/8272 is resolved.
		//{
		//	name:             "no extension and no signature",
		//	includeSignature: false,
		//	expectError:      true,
		//},
	}

	logger := log.NewTestingLogger(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			height, round := int64(1), int32(0)
			quorumHash := crypto.RandQuorumHash()
			privVal := NewMockPVForQuorum(quorumHash)
			proTxHash, err := privVal.GetProTxHash(ctx)
			require.NoError(t, err)
			pk, err := privVal.GetPubKey(ctx, quorumHash)
			require.NoError(t, err)
			blk := Block{}
			blockID, err := blk.BlockID()
			require.NoError(t, err)
			stateID := RandStateID().WithHeight(height - 1)
			vote := &Vote{
				ValidatorProTxHash: proTxHash,
				ValidatorIndex:     0,
				Height:             height,
				Round:              round,
				Type:               tmproto.PrecommitType,
				BlockID:            blockID,
				Extension:          tc.extension,
			}

			v := vote.ToProto()
			err = privVal.SignVote(ctx, "test_chain_id", btcjson.LLMQType_5_60, quorumHash, v, stateID, logger)
			require.NoError(t, err)
			vote.BlockSignature = v.BlockSignature
			vote.StateSignature = v.StateSignature
			if tc.includeSignature {
				vote.ExtensionSignature = v.ExtensionSignature
			}
			_, _, err = vote.VerifyWithExtension("test_chain_id", btcjson.LLMQType_5_60, quorumHash, pk, proTxHash, stateID)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
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

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)

	quorumType := btcjson.LLMQType_5_60

	pubkey, err := privVal.GetPubKey(context.Background(), quorumHash)
	require.NoError(t, err)

	vote := examplePrevote(t)
	vote.ValidatorProTxHash = proTxHash

	stateID := RandStateID().WithHeight(vote.Height - 1)
	_, _, err = vote.Verify("test_chain_id", quorumType, quorumHash, bls12381.GenPrivKey().PubKey(),
		crypto.RandProTxHash(), stateID)

	if assert.Error(t, err) {
		assert.Equal(t, ErrVoteInvalidValidatorProTxHash, err)
	}

	_, _, err = vote.Verify("test_chain_id", quorumType, quorumHash, pubkey, proTxHash, stateID)
	if assert.Error(t, err) {
		assert.True(
			t, strings.HasPrefix(err.Error(), ErrVoteInvalidBlockSignature.Error()),
		) // since block signatures are verified first
	}
}

func TestVoteString(t *testing.T) {
	str := examplePrecommit(t).String()
	expected := `Vote{56789:959A8F5EF2BE 12345/02/SIGNED_MSG_TYPE_PRECOMMIT(Precommit) 8B01023386C3 000000000000 000000000000 000000000000}`
	if str != expected {
		t.Errorf("got unexpected string for Vote. Expected:\n%v\nGot:\n%v", expected, str)
	}

	str2 := examplePrevote(t).String()
	expected = `Vote{56789:959A8F5EF2BE 12345/02/SIGNED_MSG_TYPE_PREVOTE(Prevote) 8B01023386C3 000000000000 000000000000 000000000000}`
	if str2 != expected {
		t.Errorf("got unexpected string for Vote. Expected:\n%v\nGot:\n%v", expected, str2)
	}
}

func signVote(
	ctx context.Context,
	t *testing.T,
	pv PrivValidator,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	vote *Vote,
	stateID StateID,
	logger log.Logger,
) {
	t.Helper()

	v := vote.ToProto()
	require.NoError(t, pv.SignVote(ctx, chainID, quorumType, quorumHash, v, stateID, logger))
	vote.StateSignature = v.StateSignature
	vote.BlockSignature = v.BlockSignature
	vote.ExtensionSignature = v.ExtensionSignature
}

func TestValidVotes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCases := []struct {
		name         string
		vote         *Vote
		malleateVote func(*Vote)
	}{
		{"good prevote", examplePrevote(t), func(v *Vote) {}},
		{"good precommit without vote extension", examplePrecommit(t), func(v *Vote) { v.Extension = nil }},
		{"good precommit with vote extension", examplePrecommit(t), func(v *Vote) { v.Extension = []byte("extension") }},
	}
	for _, tc := range testCases {
		quorumHash := crypto.RandQuorumHash()
		privVal := NewMockPVForQuorum(quorumHash)

		v := tc.vote.ToProto()
		stateID := RandStateID().WithHeight(v.Height - 1)
		signVote(ctx, t, privVal, "test_chain_id", 0, quorumHash, tc.vote, stateID, nil)
		tc.malleateVote(tc.vote)
		require.NoError(t, tc.vote.ValidateBasic(), "ValidateBasic for %s", tc.name)
		require.NoError(t, tc.vote.ValidateWithExtension(), "ValidateWithExtension for %s", tc.name)
	}
}

func TestInvalidVotes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCases := []struct {
		name         string
		malleateVote func(*Vote)
	}{
		{"negative height", func(v *Vote) { v.Height = -1 }},
		{"negative round", func(v *Vote) { v.Round = -1 }},
		{"invalid block ID", func(v *Vote) { v.BlockID = BlockID{[]byte{1, 2, 3}, PartSetHeader{111, []byte("blockparts")}} }},
		{"Invalid ProTxHash", func(v *Vote) { v.ValidatorProTxHash = make([]byte, 1) }},
		{"Invalid ValidatorIndex", func(v *Vote) { v.ValidatorIndex = -1 }},
		{"Invalid Signature", func(v *Vote) { v.BlockSignature = nil }},
		{"Too big Signature", func(v *Vote) { v.BlockSignature = make([]byte, SignatureSize+1) }},
	}
	for _, tc := range testCases {
		quorumHash := crypto.RandQuorumHash()
		privVal := NewMockPVForQuorum(quorumHash)
		prevote := examplePrevote(t)
		v := prevote.ToProto()
		stateID := RandStateID().WithHeight(v.Height - 1)
		signVote(ctx, t, privVal, "test_chain_id", 0, quorumHash, prevote, stateID, nil)
		tc.malleateVote(prevote)
		require.Error(t, prevote.ValidateBasic(), "ValidateBasic for %s in invalid prevote", tc.name)
		require.Error(t, prevote.ValidateWithExtension(), "ValidateWithExtension for %s in invalid prevote", tc.name)

		precommit := examplePrecommit(t)
		signVote(ctx, t, privVal, "test_chain_id", 0, quorumHash, precommit, stateID, nil)
		tc.malleateVote(precommit)
		require.Error(t, precommit.ValidateBasic(), "ValidateBasic for %s in invalid precommit", tc.name)
		require.Error(t, precommit.ValidateWithExtension(), "ValidateWithExtension for %s in invalid precommit", tc.name)
	}
}

func TestInvalidPrevotes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)

	testCases := []struct {
		name         string
		malleateVote func(*Vote)
	}{
		{"vote extension present", func(v *Vote) { v.Extension = []byte("extension") }},
		{"vote extension signature present", func(v *Vote) { v.ExtensionSignature = []byte("signature") }},
	}
	for _, tc := range testCases {
		prevote := examplePrevote(t)
		v := prevote.ToProto()
		stateID := RandStateID().WithHeight(v.Height - 1)
		signVote(ctx, t, privVal, "test_chain_id", 0, quorumHash, prevote, stateID, nil)
		tc.malleateVote(prevote)
		require.Error(t, prevote.ValidateBasic(), "ValidateBasic for %s", tc.name)
		require.Error(t, prevote.ValidateWithExtension(), "ValidateWithExtension for %s", tc.name)
	}
}

func TestInvalidPrecommitExtensions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)

	testCases := []struct {
		name         string
		malleateVote func(*Vote)
	}{
		{"vote extension present without signature", func(v *Vote) {
			v.Extension = []byte("extension")
			v.ExtensionSignature = nil
		}},
		// TODO(thane): Re-enable once https://github.com/tendermint/tendermint/issues/8272 is resolved
		//{"missing vote extension signature", func(v *Vote) { v.ExtensionSignature = nil }},
		{"oversized vote extension signature", func(v *Vote) { v.ExtensionSignature = make([]byte, SignatureSize+1) }},
	}
	for _, tc := range testCases {
		precommit := examplePrecommit(t)
		v := precommit.ToProto()
		stateID := RandStateID().WithHeight(v.Height - 1)
		signVote(ctx, t, privVal, "test_chain_id", 0, quorumHash, precommit, stateID, nil)
		tc.malleateVote(precommit)
		// We don't expect an error from ValidateBasic, because it doesn't
		// handle vote extensions.
		require.NoError(t, precommit.ValidateBasic(), "ValidateBasic for %s", tc.name)
		require.Error(t, precommit.ValidateWithExtension(), "ValidateWithExtension for %s", tc.name)
	}
}

func TestVoteProtobuf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)
	vote := examplePrecommit(t)
	v := vote.ToProto()
	stateID := RandStateID().WithHeight(v.Height - 1)
	err := privVal.SignVote(ctx, "test_chain_id", 0, quorumHash, v, stateID, nil)
	vote.BlockSignature = v.BlockSignature
	vote.StateSignature = v.StateSignature
	require.NoError(t, err)

	testCases := []struct {
		msg                 string
		vote                *Vote
		convertsOk          bool
		passesValidateBasic bool
	}{
		{"success", vote, true, true},
		{"fail vote validate basic", &Vote{}, true, false},
	}
	for _, tc := range testCases {
		protoProposal := tc.vote.ToProto()

		v, err := VoteFromProto(protoProposal)
		if tc.convertsOk {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}

		err = v.ValidateBasic()
		if tc.passesValidateBasic {
			require.NoError(t, err)
			require.Equal(t, tc.vote, v, tc.msg)
		} else {
			require.Error(t, err)
		}
	}
}

var sink interface{}

func BenchmarkVoteSignBytes(b *testing.B) {
	protoVote := examplePrecommit(b).ToProto()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = VoteBlockSignBytes("test_chain_id", protoVote)
	}

	if sink == nil {
		b.Fatal("Benchmark did not run")
	}

	// Reset the sink.
	sink = (interface{})(nil)
}
