package types

import (
	"github.com/dashevo/dashd-go/btcjson"
	"math"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/protoio"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	testProposal *Proposal
	pbp          *tmproto.Proposal
)

func init() {
	var stamp, err = time.Parse(TimeFormat, "2018-02-11T07:09:22.765Z")
	if err != nil {
		panic(err)
	}
	testProposal = &Proposal{
		Height:                12345,
		CoreChainLockedHeight: 100,
		Round:                 23456,
		BlockID: BlockID{Hash: []byte("--June_15_2020_amino_was_removed"),
			PartSetHeader: PartSetHeader{Total: 111, Hash: []byte("--June_15_2020_amino_was_removed")}},
		POLRound:  -1,
		Timestamp: stamp,
	}
	pbp = testProposal.ToProto()
}

func TestProposalSignable(t *testing.T) {
	chainID := "test_chain_id"
	signBytes := ProposalBlockSignBytes(chainID, pbp)
	pb := CanonicalizeProposal(chainID, pbp)

	expected, err := protoio.MarshalDelimited(&pb)
	require.NoError(t, err)
	require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Proposal")
}

func TestProposalString(t *testing.T) {
	str := testProposal.String()
	expected := `Proposal{12345/23456 (2D2D4A756E655F31355F323032305F616D696E6F5F7761735F72656D6F766564:111:2D2D4A756E65, -1) 000000000000 @ 2018-02-11T07:09:22.765Z}` //nolint:lll // ignore line length for tests
	if str != expected {
		t.Errorf("got unexpected string for Proposal. Expected:\n%v\nGot:\n%v", expected, str)
	}
}

func TestProposalVerifySignature(t *testing.T) {
	privVal := NewMockPV()
	pubKey, err := privVal.GetPubKey(crypto.QuorumHash{})
	require.NoError(t, err)

	prop := NewProposal(
		4, 1, 2, 2,
		BlockID{tmrand.Bytes(tmhash.Size), PartSetHeader{777, tmrand.Bytes(tmhash.Size)}})
	p := prop.ToProto()
	quorumHash := crypto.RandQuorumHash()
	signId := ProposalBlockSignId("test_chain_id", p, btcjson.LLMQType_5_60, quorumHash)

	// sign it
	err = privVal.SignProposal("test_chain_id", btcjson.LLMQType_5_60, quorumHash, p)
	require.NoError(t, err)
	prop.Signature = p.Signature

	// verify the same proposal
	valid := pubKey.VerifySignatureDigest(signId, prop.Signature)
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
	newSignId := ProposalBlockSignId("test_chain_id", pb, btcjson.LLMQType_5_60, quorumHash)
	require.Equal(t, string(signId), string(newSignId))
	valid = pubKey.VerifySignatureDigest(newSignId, np.Signature)
	require.True(t, valid)
}

func BenchmarkProposalWriteSignBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ProposalBlockSignBytes("test_chain_id", pbp)
	}
}

func BenchmarkProposalSign(b *testing.B) {
	privVal := NewMockPV()
	for i := 0; i < b.N; i++ {
		err := privVal.SignProposal("test_chain_id", 0, crypto.QuorumHash{}, pbp)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkProposalVerifySignature(b *testing.B) {
	privVal := NewMockPV()
	err := privVal.SignProposal("test_chain_id", 0, crypto.QuorumHash{}, pbp)
	require.NoError(b, err)
	pubKey, err := privVal.GetPubKey(crypto.QuorumHash{})
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		pubKey.VerifySignature(ProposalBlockSignBytes("test_chain_id", pbp), testProposal.Signature)
	}
}

func TestProposalValidateBasic(t *testing.T) {

	privVal := NewMockPV()
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
			p.Signature = make([]byte, MaxSignatureSize+1)
		}, true},
	}
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			prop := NewProposal(
				4, 1, 2, 2,
				blockID)
			p := prop.ToProto()
			err := privVal.SignProposal("test_chain_id", 0, crypto.QuorumHash{}, p)
			prop.Signature = p.Signature
			require.NoError(t, err)
			tc.malleateProposal(prop)
			assert.Equal(t, tc.expectErr, prop.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestProposalProtoBuf(t *testing.T) {
	proposal := NewProposal(1, 1, 2, 3, makeBlockID([]byte("hash"), 2, []byte("part_set_hash")))
	proposal.Signature = []byte("sig")
	proposal2 := NewProposal(1, 1, 2, 3, BlockID{})

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
