package privval

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
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
}

func getSignerTestCases(t *testing.T) []signerTestCase {
	testCases := make([]signerTestCase, 0)

	// Get test cases for each possible dialer (DialTCP / DialUnix / etc)
	for _, dtc := range getDialerTestCases(t) {
		chainID := tmrand.Str(12)
		mockPV := types.NewMockPV()

		// get a pair of signer listener, signer dialer endpoints
		sl, sd := getMockEndpoints(t, dtc.addr, dtc.dialer)
		sc, err := NewSignerClient(sl, chainID)
		require.NoError(t, err)
		ss := NewSignerServer(sd, chainID, btcjson.LLMQType_5_60, crypto.RandQuorumHash(), mockPV)

		err = ss.Start()
		require.NoError(t, err)

		tc := signerTestCase{
			chainID:      chainID,
			quorumType:   btcjson.LLMQType_5_60,
			quorumHash:   crypto.RandQuorumHash(),
			mockPV:       mockPV,
			signerClient: sc,
			signerServer: ss,
		}

		testCases = append(testCases, tc)
	}

	return testCases
}

func TestSignerClose(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		err := tc.signerClient.Close()
		assert.NoError(t, err)

		err = tc.signerServer.Stop()
		assert.NoError(t, err)
	}
}

func TestSignerPing(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		err := tc.signerClient.Ping()
		assert.NoError(t, err)
	}
}

func TestSignerGetPubKey(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		pubKey, err := tc.signerClient.GetPubKey(tc.quorumHash)
		require.NoError(t, err)
		expectedPubKey, err := tc.mockPV.GetPubKey(tc.quorumHash)
		require.NoError(t, err)

		assert.Equal(t, expectedPubKey, pubKey)

		pubKey, err = tc.signerClient.GetPubKey(tc.quorumHash)
		require.NoError(t, err)
		expectedpk, err := tc.mockPV.GetPubKey(tc.quorumHash)
		require.NoError(t, err)
		expectedAddr := expectedpk.Address()

		assert.Equal(t, expectedAddr, pubKey.Address())
	}
}

func TestSignerProposal(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		ts := time.Now()
		hash := tmrand.Bytes(tmhash.Size)
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

		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		require.NoError(t, tc.mockPV.SignProposal(tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto()))
		require.NoError(t, tc.signerClient.SignProposal(tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto()))

		assert.Equal(t, want.Signature, have.Signature)
	}
}

func TestSignerVote(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		hash := tmrand.Bytes(tmhash.Size)
		stateHash := tmrand.Bytes(tmhash.Size)
		valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
		want := &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			StateID:            types.StateID{LastAppHash: stateHash},
			ValidatorProTxHash: valProTxHash,
			ValidatorIndex:     1,
		}

		have := &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			StateID:            types.StateID{LastAppHash: stateHash},
			ValidatorProTxHash: valProTxHash,
			ValidatorIndex:     1,
		}

		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto()))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto()))

		assert.Equal(t, want.BlockSignature, have.BlockSignature)
		assert.Equal(t, want.StateSignature, have.StateSignature)
	}
}

func TestSignerVoteResetDeadline(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		hash := tmrand.Bytes(tmhash.Size)
		stateHash := tmrand.Bytes(tmhash.Size)
		valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
		want := &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			StateID:            types.StateID{LastAppHash: stateHash},
			ValidatorProTxHash: valProTxHash,
			ValidatorIndex:     1,
		}

		have := &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			StateID:            types.StateID{LastAppHash: stateHash},
			ValidatorProTxHash: valProTxHash,
			ValidatorIndex:     1,
		}

		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		time.Sleep(testTimeoutReadWrite2o3)

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto()))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto()))
		assert.Equal(t, want.BlockSignature, have.BlockSignature)
		assert.Equal(t, want.StateSignature, have.StateSignature)

		// TODO(jleni): Clarify what is actually being tested

		// This would exceed the deadline if it was not extended by the previous message
		time.Sleep(testTimeoutReadWrite2o3)

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto()))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto()))
		assert.Equal(t, want.BlockSignature, have.BlockSignature)
		assert.Equal(t, want.StateSignature, have.StateSignature)
	}
}

func TestSignerVoteKeepAlive(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		hash := tmrand.Bytes(tmhash.Size)
		stateHash := tmrand.Bytes(tmhash.Size)
		valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
		want := &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			StateID:            types.StateID{LastAppHash: stateHash},
			ValidatorProTxHash: valProTxHash,
			ValidatorIndex:     1,
		}

		have := &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			StateID:            types.StateID{LastAppHash: stateHash},
			ValidatorProTxHash: valProTxHash,
			ValidatorIndex:     1,
		}

		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		// Check that even if the client does not request a
		// signature for a long time. The service is still available

		// in this particular case, we use the dialer logger to ensure that
		// test messages are properly interleaved in the test logs
		tc.signerServer.Logger.Debug("TEST: Forced Wait -------------------------------------------------")
		time.Sleep(testTimeoutReadWrite * 3)
		tc.signerServer.Logger.Debug("TEST: Forced Wait DONE---------------------------------------------")

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto()))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, have.ToProto()))

		assert.Equal(t, want.BlockSignature, have.BlockSignature)
		assert.Equal(t, want.StateSignature, have.StateSignature)
	}
}

func TestSignerSignProposalErrors(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		// Replace service with a mock that always fails
		tc.signerServer.privVal = types.NewErroringMockPV()
		tc.mockPV = types.NewErroringMockPV()

		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		hash := tmrand.Bytes(tmhash.Size)
		proposal := &types.Proposal{
			Type:                  tmproto.ProposalType,
			Height:                1,
			CoreChainLockedHeight: 1,
			Round:                 2,
			POLRound:              2,
			BlockID:               types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Signature:             []byte("signature"),
		}

		err := tc.signerClient.SignProposal(tc.chainID, tc.quorumType, tc.quorumHash, proposal.ToProto())
		require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

		err = tc.mockPV.SignProposal(tc.chainID, tc.quorumType, tc.quorumHash, proposal.ToProto())
		require.Error(t, err)

		err = tc.signerClient.SignProposal(tc.chainID, tc.quorumType, tc.quorumHash, proposal.ToProto())
		require.Error(t, err)
	}
}

func TestSignerSignVoteErrors(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		hash := tmrand.Bytes(tmhash.Size)
		stateHash := tmrand.Bytes(tmhash.Size)
		valProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
		vote := &types.Vote{
			Type:               tmproto.PrecommitType,
			Height:             1,
			Round:              2,
			BlockID:            types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			StateID:            types.StateID{LastAppHash: stateHash},
			ValidatorProTxHash: valProTxHash,
			ValidatorIndex:     1,
			BlockSignature:     []byte("signature"),
			StateSignature:     []byte("stateSignature"),
		}

		// Replace signer service privval with one that always fails
		tc.signerServer.privVal = types.NewErroringMockPV()
		tc.mockPV = types.NewErroringMockPV()

		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		err := tc.signerClient.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, vote.ToProto())
		require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

		err = tc.mockPV.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, vote.ToProto())
		require.Error(t, err)

		err = tc.signerClient.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, vote.ToProto())
		require.Error(t, err)
	}
}

func brokenHandler(privVal types.PrivValidator, request privvalproto.Message,
	chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash) (privvalproto.Message, error) {
	var res privvalproto.Message
	var err error

	switch r := request.Sum.(type) {
	// This is broken and will answer most requests with a pubkey response
	case *privvalproto.Message_PubKeyRequest:
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
	for _, tc := range getSignerTestCases(t) {
		tc.signerServer.privVal = types.NewMockPV()
		tc.mockPV = types.NewMockPV()

		tc.signerServer.SetRequestHandler(brokenHandler)

		tc := tc
		t.Cleanup(func() {
			if err := tc.signerServer.Stop(); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(func() {
			if err := tc.signerClient.Close(); err != nil {
				t.Error(err)
			}
		})

		want := &types.Vote{Type: tmproto.PrecommitType}

		e := tc.signerClient.SignVote(tc.chainID, tc.quorumType, tc.quorumHash, want.ToProto())
		assert.EqualError(t, e, "empty response")
	}
}
