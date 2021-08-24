package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const ChainID = "123"

func TestGetPubKey(t *testing.T) {

	testCases := []struct {
		name string
		pv   consensus.PrivValidator
		err  bool
	}{
		{name: "valid", pv: consensus.NewMockPV(), err: false},
		{name: "error on pubkey", pv: consensus.NewErroringMockPV(), err: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := tmgrpc.NewSignerServer(ChainID, tc.pv, log.TestingLogger())

			req := &privvalproto.PubKeyRequest{ChainId: ChainID}
			resp, err := s.GetPubKey(context.Background(), req)
			if tc.err {
				require.Error(t, err)
			} else {
				pk, err := tc.pv.GetPubKey(context.Background())
				require.NoError(t, err)
				assert.Equal(t, resp.PubKey.GetEd25519(), pk.Bytes())
			}
		})
	}

}

func TestSignVote(t *testing.T) {

	ts := time.Now()
	hash := tmrand.Bytes(tmhash.Size)
	valAddr := tmrand.Bytes(crypto.AddressSize)

	testCases := []struct {
		name       string
		pv         consensus.PrivValidator
		have, want *consensus.Vote
		err        bool
	}{
		{name: "valid", pv: consensus.NewMockPV(), have: &consensus.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
		}, want: &consensus.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
		},
			err: false},
		{name: "invalid vote", pv: consensus.NewErroringMockPV(), have: &consensus.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
			Signature:        []byte("signed"),
		}, want: &consensus.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
			Signature:        []byte("signed"),
		},
			err: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := tmgrpc.NewSignerServer(ChainID, tc.pv, log.TestingLogger())

			req := &privvalproto.SignVoteRequest{ChainId: ChainID, Vote: tc.have.ToProto()}
			resp, err := s.SignVote(context.Background(), req)
			if tc.err {
				require.Error(t, err)
			} else {
				pbVote := tc.want.ToProto()

				require.NoError(t, tc.pv.SignVote(context.Background(), ChainID, pbVote))

				assert.Equal(t, pbVote.Signature, resp.Vote.Signature)
			}
		})
	}
}

func TestSignProposal(t *testing.T) {

	ts := time.Now()
	hash := tmrand.Bytes(tmhash.Size)

	testCases := []struct {
		name       string
		pv         consensus.PrivValidator
		have, want *consensus.Proposal
		err        bool
	}{
		{name: "valid", pv: consensus.NewMockPV(), have: &consensus.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
		}, want: &consensus.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
		},
			err: false},
		{name: "invalid proposal", pv: consensus.NewErroringMockPV(), have: &consensus.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
			Signature: []byte("signed"),
		}, want: &consensus.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   metadata.BlockID{Hash: hash, PartSetHeader: metadata.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
			Signature: []byte("signed"),
		},
			err: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := tmgrpc.NewSignerServer(ChainID, tc.pv, log.TestingLogger())

			req := &privvalproto.SignProposalRequest{ChainId: ChainID, Proposal: tc.have.ToProto()}
			resp, err := s.SignProposal(context.Background(), req)
			if tc.err {
				require.Error(t, err)
			} else {
				pbProposal := tc.want.ToProto()
				require.NoError(t, tc.pv.SignProposal(context.Background(), ChainID, pbProposal))
				assert.Equal(t, pbProposal.Signature, resp.Proposal.Signature)
			}
		})
	}
}
