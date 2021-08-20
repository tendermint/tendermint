package consensus_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	testProposal *consensus.Proposal
	pbp          *tmproto.Proposal
)

func init() {
	var stamp, err = time.Parse(metadata.TimeFormat, "2018-02-11T07:09:22.765Z")
	if err != nil {
		panic(err)
	}
	testProposal = &consensus.Proposal{
		Height: 12345,
		Round:  23456,
		BlockID: metadata.BlockID{Hash: []byte("--June_15_2020_amino_was_removed"),
			PartSetHeader: metadata.PartSetHeader{Total: 111, Hash: []byte("--June_15_2020_amino_was_removed")}},
		POLRound:  -1,
		Timestamp: stamp,
	}
	pbp = testProposal.ToProto()
}

func TestProposalSignable(t *testing.T) {
	chainID := "test_chain_id"
	signBytes := consensus.ProposalSignBytes(chainID, pbp)
	pb := consensus.CanonicalizeProposal(chainID, pbp)

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
	privVal := consensus.NewMockPV()
	pubKey, err := privVal.GetPubKey(context.Background())
	require.NoError(t, err)

	prop := consensus.NewProposal(
		4, 2, 2,
		metadata.BlockID{tmrand.Bytes(tmhash.Size), metadata.PartSetHeader{777, tmrand.Bytes(tmhash.Size)}})
	p := prop.ToProto()
	signBytes := consensus.ProposalSignBytes("test_chain_id", p)

	// sign it
	err = privVal.SignProposal(context.Background(), "test_chain_id", p)
	require.NoError(t, err)
	prop.Signature = p.Signature

	// verify the same proposal
	valid := pubKey.VerifySignature(signBytes, prop.Signature)
	require.True(t, valid)

	// serialize, deserialize and verify again....
	newProp := new(tmproto.Proposal)
	pb := prop.ToProto()

	bs, err := proto.Marshal(pb)
	require.NoError(t, err)

	err = proto.Unmarshal(bs, newProp)
	require.NoError(t, err)

	np, err := consensus.ProposalFromProto(newProp)
	require.NoError(t, err)

	// verify the transmitted proposal
	newSignBytes := consensus.ProposalSignBytes("test_chain_id", pb)
	require.Equal(t, string(signBytes), string(newSignBytes))
	valid = pubKey.VerifySignature(newSignBytes, np.Signature)
	require.True(t, valid)
}

func BenchmarkProposalWriteSignBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		consensus.ProposalSignBytes("test_chain_id", pbp)
	}
}

func BenchmarkProposalSign(b *testing.B) {
	privVal := consensus.NewMockPV()
	for i := 0; i < b.N; i++ {
		err := privVal.SignProposal(context.Background(), "test_chain_id", pbp)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkProposalVerifySignature(b *testing.B) {
	privVal := consensus.NewMockPV()
	err := privVal.SignProposal(context.Background(), "test_chain_id", pbp)
	require.NoError(b, err)
	pubKey, err := privVal.GetPubKey(context.Background())
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		pubKey.VerifySignature(consensus.ProposalSignBytes("test_chain_id", pbp), testProposal.Signature)
	}
}

func TestProposalValidateBasic(t *testing.T) {
	privVal := consensus.NewMockPV()
	testCases := []struct {
		testName         string
		malleateProposal func(*consensus.Proposal)
		expectErr        bool
	}{
		{"Good Proposal", func(p *consensus.Proposal) {}, false},
		{"Invalid Type", func(p *consensus.Proposal) { p.Type = tmproto.PrecommitType }, true},
		{"Invalid Height", func(p *consensus.Proposal) { p.Height = -1 }, true},
		{"Invalid Round", func(p *consensus.Proposal) { p.Round = -1 }, true},
		{"Invalid POLRound", func(p *consensus.Proposal) { p.POLRound = -2 }, true},
		{"Invalid BlockId", func(p *consensus.Proposal) {
			p.BlockID = metadata.BlockID{[]byte{1, 2, 3}, metadata.PartSetHeader{111, []byte("blockparts")}}
		}, true},
		{"Invalid Signature", func(p *consensus.Proposal) {
			p.Signature = make([]byte, 0)
		}, true},
		{"Too big Signature", func(p *consensus.Proposal) {
			p.Signature = make([]byte, metadata.MaxSignatureSize+1)
		}, true},
	}
	blockID := metadata.BlockID{tmhash.Sum([]byte("blockhash")), metadata.PartSetHeader{math.MaxInt32, tmhash.Sum([]byte("partshash"))}}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			prop := consensus.NewProposal(
				4, 2, 2,
				blockID)
			p := prop.ToProto()
			err := privVal.SignProposal(context.Background(), "test_chain_id", p)
			prop.Signature = p.Signature
			require.NoError(t, err)
			tc.malleateProposal(prop)
			assert.Equal(t, tc.expectErr, prop.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestProposalProtoBuf(t *testing.T) {
	proposal := consensus.NewProposal(1, 2, 3, metadata.BlockID{[]byte("hash"), metadata.PartSetHeader{2, []byte("part_set_hash")}})
	proposal.Signature = []byte("sig")
	proposal2 := consensus.NewProposal(1, 2, 3, metadata.BlockID{})

	testCases := []struct {
		msg     string
		p1      *consensus.Proposal
		expPass bool
	}{
		{"success", proposal, true},
		{"success", proposal2, false}, // blcokID cannot be empty
		{"empty proposal failure validatebasic", &consensus.Proposal{}, false},
		{"nil proposal", nil, false},
	}
	for _, tc := range testCases {
		protoProposal := tc.p1.ToProto()

		p, err := consensus.ProposalFromProto(protoProposal)
		if tc.expPass {
			require.NoError(t, err)
			require.Equal(t, tc.p1, p, tc.msg)
		} else {
			require.Error(t, err)
		}
	}
}
