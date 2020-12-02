package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// Unit
// todo: make into table driven tests
func TestGetPubKey(t *testing.T) {
	mockPV := types.NewMockPV()

	s := tmgrpc.SignerServer{
		ChainID: "123",
		PrivVal: mockPV,
	}

	req := &privvalproto.PubKeyRequest{ChainId: s.ChainID}
	resp, err := s.GetPubKey(context.Background(), req)
	if err != nil {
		t.Errorf("got unexpected error")
	}

	assert.Equal(t, resp.PubKey.GetEd25519(), mockPV.PrivKey.PubKey().Bytes())
}
func TestSignVote(t *testing.T) {
	mockPV := types.NewMockPV()

	s := tmgrpc.SignerServer{
		ChainID: "123",
		PrivVal: mockPV,
	}

	ts := time.Now()
	hash := tmrand.Bytes(tmhash.Size)
	valAddr := tmrand.Bytes(crypto.AddressSize)

	want := &types.Vote{
		Type:             tmproto.PrecommitType,
		Height:           1,
		Round:            2,
		BlockID:          types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
		Timestamp:        ts,
		ValidatorAddress: valAddr,
		ValidatorIndex:   1,
	}

	have := &types.Vote{
		Type:             tmproto.PrecommitType,
		Height:           1,
		Round:            2,
		BlockID:          types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
		Timestamp:        ts,
		ValidatorAddress: valAddr,
		ValidatorIndex:   1,
	}

	req := &privvalproto.SignVoteRequest{ChainId: s.ChainID, Vote: have.ToProto()}
	resp, err := s.SignVote(context.Background(), req)
	if err != nil {
		t.Errorf("got unexpected error")
	}

	pbVote := want.ToProto()

	require.NoError(t, mockPV.SignVote(s.ChainID, pbVote))

	assert.Equal(t, pbVote.Signature, resp.Vote.Signature)
}
func TestSignProposal(t *testing.T) {
	mockPV := types.NewMockPV()

	s := tmgrpc.SignerServer{
		ChainID: "123",
		PrivVal: mockPV,
	}

	ts := time.Now()
	hash := tmrand.Bytes(tmhash.Size)
	have := &types.Proposal{
		Type:      tmproto.ProposalType,
		Height:    1,
		Round:     2,
		POLRound:  2,
		BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
		Timestamp: ts,
	}
	want := &types.Proposal{
		Type:      tmproto.ProposalType,
		Height:    1,
		Round:     2,
		POLRound:  2,
		BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
		Timestamp: ts,
	}

	req := &privvalproto.SignProposalRequest{ChainId: s.ChainID, Proposal: have.ToProto()}
	resp, err := s.SignProposal(context.Background(), req)
	if err != nil {
		t.Errorf("got unexpected error")
	}

	pbProposal := want.ToProto()

	require.NoError(t, mockPV.SignProposal(s.ChainID, pbProposal))

	assert.Equal(t, pbProposal.Signature, resp.Proposal.Signature)
}
