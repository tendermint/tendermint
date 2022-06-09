package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const ChainID = "123"

func TestGetPubKey(t *testing.T) {

	testCases := []struct {
		name string
		pv   types.PrivValidator
		err  bool
	}{
		{name: "valid", pv: types.NewMockPV(), err: false},
		{name: "error on pubkey", pv: types.NewErroringMockPV(), err: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			logger := log.NewTestingLogger(t)
			s := tmgrpc.NewSignerServer(logger, ChainID, tc.pv)
			quorumHash, _ := tc.pv.GetFirstQuorumHash(ctx)
			req := &privvalproto.PubKeyRequest{ChainId: ChainID, QuorumHash: quorumHash}
			resp, err := s.GetPubKey(ctx, req)
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				quorumHash, err := tc.pv.GetFirstQuorumHash(ctx)
				require.NoError(t, err)
				pk, err := tc.pv.GetPubKey(ctx, quorumHash)
				require.NoError(t, err)
				assert.Equal(t, resp.PubKey.GetBls12381(), pk.Bytes())
			}
		})
	}

}

func TestSignVote(t *testing.T) {

	hash := tmrand.Bytes(crypto.HashSize)
	proTxHash := crypto.RandProTxHash()

	testCases := []struct {
		name       string
		pv         types.PrivValidator
		have, want *types.Vote
		err        bool
	}{
		{name: "valid", pv: types.NewMockPV(), have: &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     1,
		}, want: &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     1,
		},
			err: false},
		{name: "invalid vote", pv: types.NewErroringMockPV(), have: &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     1,
			BlockSignature:     []byte("signed"),
		}, want: &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     1,
			BlockSignature:     []byte("signed"),
		},
			err: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			logger := log.NewTestingLogger(t)

			s := tmgrpc.NewSignerServer(logger, ChainID, tc.pv)

			quorumHash, _ := tc.pv.GetFirstQuorumHash(context.Background())
			req := &privvalproto.SignVoteRequest{
				Vote:       tc.have.ToProto(),
				ChainId:    ChainID,
				QuorumType: int32(btcjson.LLMQType_5_60),
				QuorumHash: quorumHash,
				StateId: &tmproto.StateID{
					Height:      tc.have.Height - 1,
					LastAppHash: factory.RandomHash(),
				},
			}
			resp, err := s.SignVote(ctx, req)
			if tc.err {
				require.Error(t, err)
			} else {
				pbVote := tc.want.ToProto()
				require.NoError(t, tc.pv.SignVote(ctx, ChainID, btcjson.LLMQType_5_60, quorumHash,
					pbVote, types.StateID{}, log.NewTestingLogger(t)))

				assert.Equal(t, pbVote.BlockSignature, resp.Vote.BlockSignature)
			}
		})
	}
}

func TestSignProposal(t *testing.T) {

	ts := time.Now()
	hash := tmrand.Bytes(crypto.HashSize)
	quorumHash := crypto.RandQuorumHash()

	testCases := []struct {
		name       string
		pv         types.PrivValidator
		have, want *types.Proposal
		err        bool
	}{
		{name: "valid", pv: types.NewMockPVForQuorum(quorumHash), have: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
		}, want: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
		},
			err: false},
		{name: "invalid proposal", pv: types.NewErroringMockPV(), have: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
			Signature: []byte("signed"),
		}, want: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
			Signature: []byte("signed"),
		},
			err: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			logger := log.NewTestingLogger(t)

			s := tmgrpc.NewSignerServer(logger, ChainID, tc.pv)

			req := &privvalproto.SignProposalRequest{
				Proposal:   tc.have.ToProto(),
				ChainId:    ChainID,
				QuorumType: int32(btcjson.LLMQType_5_60),
				QuorumHash: quorumHash,
			}
			resp, err := s.SignProposal(ctx, req)
			if tc.err {
				require.Error(t, err)
			} else {
				pbProposal := tc.want.ToProto()
				_, err = tc.pv.SignProposal(ctx, ChainID, btcjson.LLMQType_5_60, quorumHash, pbProposal)
				require.NoError(t, err)
				assert.Equal(t, pbProposal.Signature, resp.Proposal.Signature)
			}
		})
	}
}
