package privval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type signerTestCase struct {
	chainID      string
	mockPV       types.PrivValidator
	signerClient *SignerClient
	signerServer *SignerServer
	name         string
	closer       context.CancelFunc
}

func getSignerTestCases(ctx context.Context, t *testing.T, logger log.Logger) []signerTestCase {
	t.Helper()

	testCases := make([]signerTestCase, 0)

	// Get test cases for each possible dialer (DialTCP / DialUnix / etc)
	for idx, dtc := range getDialerTestCases(t) {
		chainID := tmrand.Str(12)
		mockPV := types.NewMockPV()

		cctx, ccancel := context.WithCancel(ctx)
		// get a pair of signer listener, signer dialer endpoints
		sl, sd := getMockEndpoints(cctx, t, logger, dtc.addr, dtc.dialer)
		sc, err := NewSignerClient(cctx, sl, chainID)
		require.NoError(t, err)
		ss := NewSignerServer(sd, chainID, mockPV)

		require.NoError(t, ss.Start(cctx))

		testCases = append(testCases, signerTestCase{
			name:         fmt.Sprintf("Case%d%T_%s", idx, dtc.dialer, chainID),
			closer:       ccancel,
			chainID:      chainID,
			mockPV:       mockPV,
			signerClient: sc,
			signerServer: ss,
		})
		t.Cleanup(ss.Wait)
		t.Cleanup(sc.endpoint.Wait)
	}

	return testCases
}

func TestSignerClose(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(bctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer leaktest.Check(t)
			defer func() {
				tc.closer()
				tc.signerClient.endpoint.Wait()
				tc.signerServer.Wait()
			}()

			assert.NoError(t, tc.signerClient.Close())
			tc.signerServer.Stop()
		})
	}
}

func TestSignerPing(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		err := tc.signerClient.Ping(ctx)
		assert.NoError(t, err)
	}
}

func TestSignerGetPubKey(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.closer()

			pubKey, err := tc.signerClient.GetPubKey(ctx)
			require.NoError(t, err)
			expectedPubKey, err := tc.mockPV.GetPubKey(ctx)
			require.NoError(t, err)

			assert.Equal(t, expectedPubKey, pubKey)

			pubKey, err = tc.signerClient.GetPubKey(ctx)
			require.NoError(t, err)
			expectedpk, err := tc.mockPV.GetPubKey(ctx)
			require.NoError(t, err)
			expectedAddr := expectedpk.Address()

			assert.Equal(t, expectedAddr, pubKey.Address())
		})
	}
}

func TestSignerProposal(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.closer()

			ts := time.Now()
			hash := tmrand.Bytes(crypto.HashSize)
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

			require.NoError(t, tc.mockPV.SignProposal(ctx, tc.chainID, want.ToProto()))
			require.NoError(t, tc.signerClient.SignProposal(ctx, tc.chainID, have.ToProto()))

			assert.Equal(t, want.Signature, have.Signature)
		})

	}
}

func TestSignerVote(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.closer()

			ts := time.Now()
			hash := tmrand.Bytes(crypto.HashSize)
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

			require.NoError(t, tc.mockPV.SignVote(ctx, tc.chainID, want.ToProto()))
			require.NoError(t, tc.signerClient.SignVote(ctx, tc.chainID, have.ToProto()))

			assert.Equal(t, want.Signature, have.Signature)
		})
	}
}

func TestSignerVoteResetDeadline(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			ts := time.Now()
			hash := tmrand.Bytes(crypto.HashSize)
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

			time.Sleep(testTimeoutReadWrite2o3)

			require.NoError(t, tc.mockPV.SignVote(ctx, tc.chainID, want.ToProto()))
			require.NoError(t, tc.signerClient.SignVote(ctx, tc.chainID, have.ToProto()))
			assert.Equal(t, want.Signature, have.Signature)

			// TODO(jleni): Clarify what is actually being tested

			// This would exceed the deadline if it was not extended by the previous message
			time.Sleep(testTimeoutReadWrite2o3)

			require.NoError(t, tc.mockPV.SignVote(ctx, tc.chainID, want.ToProto()))
			require.NoError(t, tc.signerClient.SignVote(ctx, tc.chainID, have.ToProto()))
			assert.Equal(t, want.Signature, have.Signature)
		})
	}
}

func TestSignerVoteKeepAlive(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.closer()

			ts := time.Now()
			hash := tmrand.Bytes(crypto.HashSize)
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

			// Check that even if the client does not request a
			// signature for a long time. The service is still available

			// in this particular case, we use the dialer logger to ensure that
			// test messages are properly interleaved in the test logs
			time.Sleep(testTimeoutReadWrite * 3)

			require.NoError(t, tc.mockPV.SignVote(ctx, tc.chainID, want.ToProto()))
			require.NoError(t, tc.signerClient.SignVote(ctx, tc.chainID, have.ToProto()))

			assert.Equal(t, want.Signature, have.Signature)
		})
	}
}

func TestSignerSignProposalErrors(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.closer()
			// Replace service with a mock that always fails
			tc.signerServer.privVal = types.NewErroringMockPV()
			tc.mockPV = types.NewErroringMockPV()

			ts := time.Now()
			hash := tmrand.Bytes(crypto.HashSize)
			proposal := &types.Proposal{
				Type:      tmproto.ProposalType,
				Height:    1,
				Round:     2,
				POLRound:  2,
				BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				Timestamp: ts,
				Signature: []byte("signature"),
			}

			err := tc.signerClient.SignProposal(ctx, tc.chainID, proposal.ToProto())
			rserr, ok := err.(*RemoteSignerError)
			require.True(t, ok, "%T", err)
			require.Contains(t, rserr.Error(), types.ErroringMockPVErr.Error())

			err = tc.mockPV.SignProposal(ctx, tc.chainID, proposal.ToProto())
			require.Error(t, err)

			err = tc.signerClient.SignProposal(ctx, tc.chainID, proposal.ToProto())
			require.Error(t, err)
		})
	}
}

func TestSignerSignVoteErrors(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.closer()

			ts := time.Now()
			hash := tmrand.Bytes(crypto.HashSize)
			valAddr := tmrand.Bytes(crypto.AddressSize)
			vote := &types.Vote{
				Type:             tmproto.PrecommitType,
				Height:           1,
				Round:            2,
				BlockID:          types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				Timestamp:        ts,
				ValidatorAddress: valAddr,
				ValidatorIndex:   1,
				Signature:        []byte("signature"),
			}

			// Replace signer service privval with one that always fails
			tc.signerServer.privVal = types.NewErroringMockPV()
			tc.mockPV = types.NewErroringMockPV()

			err := tc.signerClient.SignVote(ctx, tc.chainID, vote.ToProto())
			rserr, ok := err.(*RemoteSignerError)
			require.True(t, ok, "%T", err)
			require.Contains(t, rserr.Error(), types.ErroringMockPVErr.Error())

			err = tc.mockPV.SignVote(ctx, tc.chainID, vote.ToProto())
			require.Error(t, err)

			err = tc.signerClient.SignVote(ctx, tc.chainID, vote.ToProto())
			require.Error(t, err)
		})
	}
}

func brokenHandler(ctx context.Context, privVal types.PrivValidator, request privvalproto.Message,
	chainID string) (privvalproto.Message, error) {
	var res privvalproto.Message
	var err error

	switch r := request.Sum.(type) {
	// This is broken and will answer most requests with a pubkey response
	case *privvalproto.Message_PubKeyRequest:
		res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: cryptoproto.PublicKey{}, Error: nil})
	case *privvalproto.Message_SignVoteRequest:
		res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: cryptoproto.PublicKey{}, Error: nil})
	case *privvalproto.Message_SignProposalRequest:
		res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: cryptoproto.PublicKey{}, Error: nil})
	case *privvalproto.Message_PingRequest:
		err, res = nil, mustWrapMsg(&privvalproto.PingResponse{})
	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}

func TestSignerUnexpectedResponse(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	for _, tc := range getSignerTestCases(ctx, t, logger) {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.closer()

			tc.signerServer.privVal = types.NewMockPV()
			tc.mockPV = types.NewMockPV()

			tc.signerServer.SetRequestHandler(brokenHandler)

			ts := time.Now()
			want := &types.Vote{Timestamp: ts, Type: tmproto.PrecommitType}

			e := tc.signerClient.SignVote(ctx, tc.chainID, want.ToProto())
			assert.EqualError(t, e, "empty response")
		})
	}
}
