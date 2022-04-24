package types

import (
	"context"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func getTestProposal(t testing.TB) *Proposal {
	t.Helper()

	stamp, err := time.Parse(TimeFormat, "2018-02-11T07:09:22.765Z")
	require.NoError(t, err)

	return &Proposal{
		Height: 12345,
		Round:  23456,
		BlockID: BlockID{Hash: []byte("--June_15_2020_amino_was_removed"),
			PartSetHeader: PartSetHeader{Total: 111, Hash: []byte("--June_15_2020_amino_was_removed")}},
		POLRound:  -1,
		Timestamp: stamp,

		CoreChainLockedHeight: 100,
	}
}

func TestProposalSignable(t *testing.T) {
	chainID := "test_chain_id"
	signBytes := ProposalBlockSignBytes(chainID, getTestProposal(t).ToProto())
	pb := CanonicalizeProposal(chainID, getTestProposal(t).ToProto())

	expected, err := protoio.MarshalDelimited(&pb)
	require.NoError(t, err)
	require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Proposal")
}

func TestProposalString(t *testing.T) {
	str := getTestProposal(t).String()
	expected := `Proposal{12345/23456 (2D2D4A756E655F31355F323032305F616D696E6F5F7761735F72656D6F766564:111:2D2D4A756E65, -1) 000000000000 @ 2018-02-11T07:09:22.765Z}`
	if str != expected {
		t.Errorf("got unexpected string for Proposal. Expected:\n%v\nGot:\n%v", expected, str)
	}
}

func TestProposalVerifySignature(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)
	pubKey, err := privVal.GetPubKey(ctx, quorumHash)
	require.NoError(t, err)

	prop := NewProposal(
		4, 1, 2, 2,
		BlockID{tmrand.Bytes(crypto.HashSize), PartSetHeader{777, tmrand.Bytes(crypto.HashSize)}},
		tmtime.Now(),
	)
	p := prop.ToProto()
	signID := ProposalBlockSignID("test_chain_id", p, btcjson.LLMQType_5_60, quorumHash)

	// sign it
	_, err = privVal.SignProposal(ctx, "test_chain_id", btcjson.LLMQType_5_60, quorumHash, p)
	require.NoError(t, err)
	prop.Signature = p.Signature

	// verify the same proposal
	valid := pubKey.VerifySignatureDigest(signID, prop.Signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	newProp := new(tmproto.Proposal)
	pb := prop.ToProto()

	bs, err := proto.Marshal(pb)
	require.NoError(t, err)

	err = proto.Unmarshal(bs, newProp)
	require.NoError(t, err)

	np, err := ProposalFromProto(newProp)
	require.NoError(t, err)

	// verify the transmitted proposal
	newSignID := ProposalBlockSignID("test_chain_id", pb, btcjson.LLMQType_5_60, quorumHash)
	require.Equal(t, string(signID), string(newSignID))
	valid = pubKey.VerifySignatureDigest(newSignID, np.Signature)
	require.True(t, valid)
}

func decodeHex(s string) tmbytes.HexBytes {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		panic("cannot decode hex: " + err.Error())
	}
	return decoded
}

func TestProposalVerifySignatureHardcoded(t *testing.T) {
	testCases := []struct {
		quorumHash tmbytes.HexBytes
		publicKey  tmbytes.HexBytes
		signID     tmbytes.HexBytes
		signature  tmbytes.HexBytes
		valid      bool
	}{
		{
			quorumHash: decodeHex("3F8093C7DAD7FAD40651B186AE11B433101DB0318B4180E4E15EBD02EC035DB2"),
			publicKey: decodeHex("972d08a96ce38e7f2de4d5186d84c7a8236854f396500fb1de963fc79464c0968150" +
				"8c0b8a86177d219514e6c55bd223"),
			signID: decodeHex("B907E1B1D6CF171B310DBC6120F3104379DE3CD3C5B5F1E5FAD9C3F7E4D5D3C0"),
			signature: decodeHex("8C4CEC8DE50D990831009B62704AC6A9E76837DFD0D492D195F2417AEFFEEC299ADE" +
				"5F8FCB96ADBCD539ED7A67BD3C0513F42FC3C6F82D5B8854D859539EBABF6D371DE98EBAC3DEBDFDD5226" +
				"9478F85968B27EFF14B2873736271D7808DEF35"),
			valid: true,
		},
		{
			publicKey: decodeHex("0eb0efcf0090d407a1c4339c0713a3be30852bc8274bc217d0ba59d12f5796af1be06" +
				"f82b59cf3f3f598d542e2816148"),
			signID: decodeHex("03A85F77715E4314B861F13405ACB2B0A9CE2A4174DD26E7BCD09F38513166D4"),
			signature: decodeHex("941BA28D7AFE968FF0570835498623F3B2C89A695F23035468E06AB622F5CD72F53C9" +
				"8951F9022C89C5A56E3BD023BF00BF04E1A6D0101179FE0B27E7594E4EC4CC56C1A8D6BBFD4E10E01752B2" +
				"7BF2B6FCC24FF07854C6DFE6F27B662C4128B"),
			valid: true,
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			pubKey := bls12381.PubKey(tc.publicKey)
			valid := pubKey.VerifySignatureDigest(tc.signID, tc.signature)
			assert.Equal(t, tc.valid, valid, "signature validation result")
		})
	}
}

func BenchmarkProposalWriteSignBytes(b *testing.B) {
	pbp := getTestProposal(b).ToProto()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ProposalBlockSignBytes("test_chain_id", pbp)
	}
}

func BenchmarkProposalSign(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)

	pbp := getTestProposal(b).ToProto()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := privVal.SignProposal(ctx, "test_chain_id", 0, quorumHash, pbp)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkProposalVerifySignature(b *testing.B) {
	testProposal := getTestProposal(b)
	pbp := testProposal.ToProto()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)
	_, err := privVal.SignProposal(ctx, "test_chain_id", 0, quorumHash, pbp)
	require.NoError(b, err)
	pubKey, err := privVal.GetPubKey(ctx, quorumHash)
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pubKey.VerifySignature(ProposalBlockSignBytes("test_chain_id", pbp), testProposal.Signature)
	}
}

func TestProposalValidateBasic(t *testing.T) {
	quorumHash := crypto.RandQuorumHash()
	privVal := NewMockPVForQuorum(quorumHash)
	testCases := []struct {
		testName         string
		malleateProposal func(*Proposal)
		expectErr        bool
	}{
		{"Good Proposal", func(p *Proposal) {}, false},
		{"Invalid Type", func(p *Proposal) { p.Type = tmproto.PrecommitType }, true},
		{"Invalid Height", func(p *Proposal) { p.Height = -1 }, true},
		{"Invalid Round", func(p *Proposal) { p.Round = -1 }, true},
		{"Invalid POLRound", func(p *Proposal) { p.POLRound = -2 }, true},
		{"Invalid BlockId", func(p *Proposal) {
			p.BlockID = BlockID{[]byte{1, 2, 3}, PartSetHeader{111, []byte("blockparts")}}
		}, true},
		{"Invalid Signature", func(p *Proposal) {
			p.Signature = make([]byte, 0)
		}, true},
		{"Too big Signature", func(p *Proposal) {
			p.Signature = make([]byte, SignatureSize+1)
		}, true},
	}
	blockID := makeBlockID(crypto.Checksum([]byte("blockhash")), math.MaxInt32, crypto.Checksum([]byte("partshash")))

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			prop := NewProposal(
				4, 1, 2, 2,
				blockID, tmtime.Now())
			p := prop.ToProto()
			_, err := privVal.SignProposal(ctx, "test_chain_id", 0, quorumHash, p)
			prop.Signature = p.Signature
			require.NoError(t, err)
			tc.malleateProposal(prop)
			assert.Equal(t, tc.expectErr, prop.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestProposalProtoBuf(t *testing.T) {
	proposal := NewProposal(1, 1, 2, 3, makeBlockID([]byte("hash"), 2, []byte("part_set_hash")), tmtime.Now())
	proposal.Signature = []byte("sig")
	proposal2 := NewProposal(1, 1, 2, 3, BlockID{}, tmtime.Now())

	testCases := []struct {
		msg     string
		p1      *Proposal
		expPass bool
	}{
		{"success", proposal, true},
		{"success", proposal2, false}, // blcokID cannot be empty
		{"empty proposal failure validatebasic", &Proposal{}, false},
		{"nil proposal", nil, false},
	}
	for _, tc := range testCases {
		protoProposal := tc.p1.ToProto()

		p, err := ProposalFromProto(protoProposal)
		if tc.expPass {
			require.NoError(t, err)
			require.Equal(t, tc.p1, p, tc.msg)
		} else {
			require.Error(t, err)
		}
	}
}

func TestIsTimely(t *testing.T) {
	genesisTime, err := time.Parse(time.RFC3339, "2019-03-13T23:00:00Z")
	require.NoError(t, err)
	testCases := []struct {
		name         string
		proposalTime time.Time
		recvTime     time.Time
		precision    time.Duration
		msgDelay     time.Duration
		expectTimely bool
		round        int32
	}{
		// proposalTime - precision <= localTime <= proposalTime + msgDelay + precision
		{
			// Checking that the following inequality evaluates to true:
			// 0 - 2 <= 1 <= 0 + 1 + 2
			name:         "basic timely",
			proposalTime: genesisTime,
			recvTime:     genesisTime.Add(1 * time.Nanosecond),
			precision:    time.Nanosecond * 2,
			msgDelay:     time.Nanosecond,
			expectTimely: true,
		},
		{
			// Checking that the following inequality evaluates to false:
			// 0 - 2 <= 4 <= 0 + 1 + 2
			name:         "local time too large",
			proposalTime: genesisTime,
			recvTime:     genesisTime.Add(4 * time.Nanosecond),
			precision:    time.Nanosecond * 2,
			msgDelay:     time.Nanosecond,
			expectTimely: false,
		},
		{
			// Checking that the following inequality evaluates to false:
			// 4 - 2 <= 0 <= 4 + 2 + 1
			name:         "proposal time too large",
			proposalTime: genesisTime.Add(4 * time.Nanosecond),
			recvTime:     genesisTime,
			precision:    time.Nanosecond * 2,
			msgDelay:     time.Nanosecond,
			expectTimely: false,
		},
		{
			// Checking that the following inequality evaluates to true:
			// 0 - (2 * 2)  <= 4 <= 0 + (1 * 2) + 2
			name:         "message delay adapts after 10 rounds",
			proposalTime: genesisTime,
			recvTime:     genesisTime.Add(4 * time.Nanosecond),
			precision:    time.Nanosecond * 2,
			msgDelay:     time.Nanosecond,
			expectTimely: true,
			round:        10,
		},
		{
			// check that values that overflow time.Duration still correctly register
			// as timely when round relaxation applied.
			name:         "message delay fixed to not overflow time.Duration",
			proposalTime: genesisTime,
			recvTime:     genesisTime.Add(4 * time.Nanosecond),
			precision:    time.Nanosecond * 2,
			msgDelay:     time.Nanosecond,
			expectTimely: true,
			round:        5000,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			p := Proposal{
				Timestamp: testCase.proposalTime,
			}

			sp := SynchronyParams{
				Precision:    testCase.precision,
				MessageDelay: testCase.msgDelay,
			}

			ti := p.IsTimely(testCase.recvTime, sp, testCase.round)
			assert.Equal(t, testCase.expectTimely, ti)
		})
	}
}
