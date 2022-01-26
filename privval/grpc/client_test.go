package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const chainID = "chain-id"

func dialer(pv types.PrivValidator, logger log.Logger) (*grpc.Server, func(context.Context, string) (net.Conn, error)) {
	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer()

	s := tmgrpc.NewSignerServer(chainID, pv, logger)

	privvalproto.RegisterPrivValidatorAPIServer(server, s)

	go func() {
		if err := server.Serve(listener); err != nil {
			panic(err)
		}
	}()

	return server, func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func TestSignerClient_GetPubKey(t *testing.T) {

	ctx := context.Background()
	mockPV := types.NewMockPV()
	logger := log.TestingLogger()
	srv, dialer := dialer(mockPV, logger)
	defer srv.Stop()

	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	quorumHash, err := mockPV.GetFirstQuorumHash(ctx)
	require.NoError(t, err)

	client, err := tmgrpc.NewSignerClient(conn, chainID, logger)
	require.NoError(t, err)
	privKey := mockPV.PrivateKeys[quorumHash.String()].PrivKey
	pk, err := client.GetPubKey(context.Background(), quorumHash)
	require.NoError(t, err)
	assert.Equal(t, privKey.PubKey(), pk)
}

func TestSignerClient_SignVote(t *testing.T) {

	ctx := context.Background()
	mockPV := types.NewMockPV()
	logger := log.TestingLogger()
	srv, dialer := dialer(mockPV, logger)
	defer srv.Stop()

	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client, err := tmgrpc.NewSignerClient(conn, chainID, logger)
	require.NoError(t, err)

	hash := tmrand.Bytes(tmhash.Size)
	valAddr := crypto.RandProTxHash()

	want := &types.Vote{
		Type:               tmproto.PrecommitType,
		Height:             1,
		Round:              2,
		BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
		ValidatorProTxHash: valAddr,
		ValidatorIndex:     1,
	}

	have := &types.Vote{
		Type:               tmproto.PrecommitType,
		Height:             1,
		Round:              2,
		BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
		ValidatorProTxHash: valAddr,
		ValidatorIndex:     1,
	}

	pbHave := have.ToProto()
	stateID := types.StateID{
		Height:      0,
		LastAppHash: nil,
	}

	quorumHash, err := mockPV.GetProTxHash(ctx)
	require.NoError(t, err)

	err = client.SignVote(ctx, chainID, btcjson.LLMQType_5_60, quorumHash, pbHave, stateID, logger)
	require.NoError(t, err)

	pbWant := want.ToProto()

	require.NoError(t, mockPV.SignVote(ctx, chainID, btcjson.LLMQType_5_60, quorumHash, pbWant, stateID, logger))

	assert.Equal(t, pbWant.StateSignature, pbHave.StateSignature)
	assert.Equal(t, pbWant.BlockSignature, pbHave.BlockSignature)
}

func TestSignerClient_SignProposal(t *testing.T) {

	ctx := context.Background()
	mockPV := types.NewMockPV()
	logger := log.TestingLogger()
	srv, dialer := dialer(mockPV, logger)
	defer srv.Stop()

	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client, err := tmgrpc.NewSignerClient(conn, chainID, logger)
	require.NoError(t, err)

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

	quorumHash, err := mockPV.GetProTxHash(ctx)
	require.NoError(t, err)

	pbHave := have.ToProto()

	_, err = client.SignProposal(context.Background(), chainID, btcjson.LLMQType_5_60, quorumHash, pbHave)
	require.NoError(t, err)

	pbWant := want.ToProto()

	_, err = mockPV.SignProposal(context.Background(), chainID, btcjson.LLMQType_5_60, quorumHash, pbWant)
	require.NoError(t, err)

	assert.Equal(t, pbWant.Signature, pbHave.Signature)
}
