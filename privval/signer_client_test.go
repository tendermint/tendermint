package privval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
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
	quorumType   btcjson.LLMQType
	quorumHash   crypto.QuorumHash
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
		quorumHash := crypto.RandQuorumHash()
		mockPV := types.NewMockPVForQuorum(quorumHash)

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
			quorumType:   btcjson.LLMQType_5_60,
			quorumHash:   quorumHash,
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

			pubKey, err := tc.signerClient.GetPubKey(ctx, tc.quorumHash)
			require.NoError(t, err)
			expectedPubKey, err := tc.mockPV.GetPubKey(ctx, tc.quorumHash)
			require.NoError(t, err)

			assert.Equal(t, expectedPubKey, pubKey)

			pubKey, err = tc.signerClient.GetPubKey(ctx, tc.quorumHash)
			require.NoError(t, err)
			expectedpk, err := tc.mockPV.GetPubKey(ctx, tc.quorumHash)
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
				Type:                  tmproto.ProposalType,
				Height:                1,
				CoreChainLockedHeight: 1,
				Round:                 2,
				POLRound:              2,
				BlockID:               types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				Timestamp:             ts,
			}
			want := &types.Proposal{
				Type:                  tmproto.ProposalType,
				Height:                1,
				CoreChainLockedHeight: 1,
				Round:                 2,
				POLRound:              2,
				BlockID:               types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				Timestamp:             ts,
			}

			_, err := tc.mockPV.SignProposal(ctx, tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto())
			require.NoError(t, err)
			_, err = tc.signerClient.SignProposal(ctx, tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto())
			require.NoError(t, err)

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

			hash := tmrand.Bytes(crypto.HashSize)
			valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
			want := &types.Vote{
				Type:               tmproto.PrecommitType,
				Height:             1,
				Round:              2,
				BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				ValidatorProTxHash: valProTxHash,
				ValidatorIndex:     1,
			}

			have := &types.Vote{
				Type:               tmproto.PrecommitType,
				Height:             1,
				Round:              2,
				BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				ValidatorProTxHash: valProTxHash,
				ValidatorIndex:     1,
			}

			stateID := types.RandStateID().WithHeight(want.Height - 1)

			require.NoError(t, tc.mockPV.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto(), stateID, nil))
			require.NoError(t, tc.signerClient.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto(), stateID, nil))

			assert.Equal(t, want.BlockSignature, have.BlockSignature)
			assert.Equal(t, want.StateSignature, have.StateSignature)

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
			hash := tmrand.Bytes(crypto.HashSize)
			valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
			want := &types.Vote{
				Type:               tmproto.PrecommitType,
				Height:             1,
				Round:              2,
				BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				ValidatorProTxHash: valProTxHash,
				ValidatorIndex:     1,
			}

			have := &types.Vote{
				Type:               tmproto.PrecommitType,
				Height:             1,
				Round:              2,
				BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				ValidatorProTxHash: valProTxHash,
				ValidatorIndex:     1,
			}

			time.Sleep(testTimeoutReadWrite2o3)

			stateID := types.RandStateID().WithHeight(want.Height - 1)

			require.NoError(t,
				tc.mockPV.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto(), stateID, nil))
			require.NoError(t,
				tc.signerClient.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto(), stateID, nil))
			assert.Equal(t, want.BlockSignature, have.BlockSignature)
			assert.Equal(t, want.StateSignature, have.StateSignature)

			// TODO(jleni): Clarify what is actually being tested

			// This would exceed the deadline if it was not extended by the previous message
			time.Sleep(testTimeoutReadWrite2o3)

			require.NoError(t,
				tc.mockPV.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto(), stateID, nil))
			require.NoError(t,
				tc.signerClient.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto(), stateID, nil))
			assert.Equal(t, want.BlockSignature, have.BlockSignature)
			assert.Equal(t, want.StateSignature, have.StateSignature)
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

			hash := tmrand.Bytes(crypto.HashSize)
			valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
			want := &types.Vote{
				Type:               tmproto.PrecommitType,
				Height:             1,
				Round:              2,
				BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				ValidatorProTxHash: valProTxHash,
				ValidatorIndex:     1,
			}

			have := &types.Vote{
				Type:               tmproto.PrecommitType,
				Height:             1,
				Round:              2,
				BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				ValidatorProTxHash: valProTxHash,
				ValidatorIndex:     1,
			}

			stateID := types.RandStateID().WithHeight(want.Height - 1)

			// Check that even if the client does not request a
			// signature for a long time. The service is still available

			// in this particular case, we use the dialer logger to ensure that
			// test messages are properly interleaved in the test logs
			time.Sleep(testTimeoutReadWrite * 3)

			require.NoError(t,
				tc.mockPV.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto(), stateID, nil))
			require.NoError(t,
				tc.signerClient.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto(), stateID, nil))

			assert.Equal(t, want.BlockSignature, have.BlockSignature)
			assert.Equal(t, want.StateSignature, have.StateSignature)
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

			hash := tmrand.Bytes(crypto.HashSize)
			proposal := &types.Proposal{
				Type:                  tmproto.ProposalType,
				Height:                1,
				CoreChainLockedHeight: 1,
				Round:                 2,
				POLRound:              2,
				BlockID:               types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				Signature:             []byte("signature"),
			}

			_, err := tc.signerClient.SignProposal(ctx, tc.chainID, tc.quorumType, tc.quorumHash, proposal.ToProto())
			rserr, ok := err.(*RemoteSignerError)
			require.True(t, ok, "%T", err)
			require.Contains(t, rserr.Error(), types.ErroringMockPVErr.Error())

			_, err = tc.mockPV.SignProposal(ctx, tc.chainID, tc.quorumType, tc.quorumHash, proposal.ToProto())
			require.Error(t, err)

			_, err = tc.signerClient.SignProposal(ctx, tc.chainID, tc.quorumType, tc.quorumHash, proposal.ToProto())
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

			hash := tmrand.Bytes(crypto.HashSize)
			valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
			vote := &types.Vote{
				Type:               tmproto.PrecommitType,
				Height:             1,
				Round:              2,
				BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
				ValidatorProTxHash: valProTxHash,
				ValidatorIndex:     1,
				BlockSignature:     []byte("signature"),
				StateSignature:     []byte("stateSignature"),
			}

			stateID := types.RandStateID().WithHeight(vote.Height - 1)

			// Replace signer service privval with one that always fails
			tc.signerServer.privVal = types.NewErroringMockPV()
			tc.mockPV = types.NewErroringMockPV()

			err := tc.signerClient.SignVote(context.Background(), tc.chainID, tc.quorumType, tc.quorumHash, vote.ToProto(), stateID, nil)
			rserr, ok := err.(*RemoteSignerError)
			require.True(t, ok, "%T", err)
			require.Contains(t, rserr.Error(), types.ErroringMockPVErr.Error())

			err = tc.mockPV.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, vote.ToProto(), stateID, nil)
			require.Error(t, err)

			err = tc.signerClient.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, vote.ToProto(), stateID, nil)
			require.Error(t, err)
		})
	}
}

func brokenHandler(ctx context.Context, privVal types.PrivValidator, request privvalproto.Message, chainID string) (privvalproto.Message, error) {
	var res privvalproto.Message
	var err error

	switch r := request.Sum.(type) {
	// This is broken and will answer most requests with a pubkey response
	case *privvalproto.Message_PubKeyRequest:
		res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: cryptoproto.PublicKey{}, Error: nil})
	case *privvalproto.Message_ThresholdPubKeyRequest:
		res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: cryptoproto.PublicKey{}, Error: nil})
	case *privvalproto.Message_ProTxHashRequest:
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

			tc.signerServer.privVal = types.NewMockPVForQuorum(tc.quorumHash)
			tc.mockPV = types.NewMockPVForQuorum(tc.quorumHash)

			tc.signerServer.SetRequestHandler(brokenHandler)

			want := &types.Vote{Type: tmproto.PrecommitType, Height: 1}

			stateID := types.RandStateID().WithHeight(want.Height - 1)

			e := tc.signerClient.SignVote(ctx, tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto(), stateID, nil)
			assert.EqualError(t, e, "empty response")
		})
	}
}
