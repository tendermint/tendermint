package grpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// Unit
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

// Integration
const bufSize = 1024 * 1024

var lis *bufconn.Listener

func init() {
	lis = bufconn.Listen(bufSize)
	logger := log.TestingLogger()
	chainID := tmrand.Str(12)
	mockPV := types.NewMockPV()

	s := grpc.NewServer()
	ss := tmgrpc.NewSignerServer(chainID, mockPV, logger)
	privvalproto.RegisterPrivValidatorAPIServer(s, ss)

	go func() {
		if err := s.Serve(lis); err != nil {
			panic(fmt.Sprintf("Server exited with error: %v", err))
		}
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

// func TestGetPubKeySigner(t *testing.T) {
// 	ctx := context.Background()
// 	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
// 	if err != nil {
// 		t.Fatalf("Failed to dial bufnet: %v", err)
// 	}
// 	defer conn.Close()
// 	client := privvalproto.NewPrivValidatorAPIClient(conn)

// 	// for _, tc := range getSignerTestCases(t) {
// 	// 	tc := tc
// 	// 	t.Cleanup(func() {
// 	// 		if err := tc.signerClient.Close(); err != nil {
// 	// 			t.Error(err)
// 	// 		}
// 	// 	})

// 	// 	fmt.Println(1)
// 	// 	err := tc.signerServer.Start()
// 	// 	require.NoError(t, err)
// 	// 	fmt.Println(2)

// 	// 	pubKey, err := tc.signerClient.GetPubKey()
// 	// 	require.NoError(t, err)
// 	// 	expectedPubKey, err := tc.mockPV.GetPubKey()
// 	// 	require.NoError(t, err)

// 	// 	assert.Equal(t, expectedPubKey, pubKey)

// 	// 	pubKey, err = tc.signerClient.GetPubKey()
// 	// 	require.NoError(t, err)
// 	// 	expectedpk, err := tc.mockPV.GetPubKey()
// 	// 	require.NoError(t, err)
// 	// 	expectedAddr := expectedpk.Address()

// 	// 	assert.Equal(t, expectedAddr, pubKey.Address())
// 	// }
// }
