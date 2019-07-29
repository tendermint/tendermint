package privval

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
)

type signerTestCase struct {
	chainID      string
	mockPV       types.PrivValidator
	signerClient *SignerClient
	signerServer *SignerServer
}

func getSignerTestCases(t *testing.T) []signerTestCase {
	testCases := make([]signerTestCase, 0)

	// Get test cases for each possible dialer (DialTCP / DialUnix / etc)
	for _, dtc := range getDialerTestCases(t) {
		chainID := common.RandStr(12)
		mockPV := types.NewMockPV()

		// get a pair of signer listener, signer dialer endpoints
		sl, sd := getMockEndpoints(t, dtc.addr, dtc.dialer)
		sc, err := NewSignerClient(sl)
		require.NoError(t, err)
		ss := NewSignerServer(sd, chainID, mockPV)

		err = ss.Start()
		require.NoError(t, err)

		tc := signerTestCase{
			chainID:      chainID,
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
		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		err := tc.signerClient.Ping()
		assert.NoError(t, err)
	}
}

func TestSignerGetPubKey(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		pubKey := tc.signerClient.GetPubKey()
		expectedPubKey := tc.mockPV.GetPubKey()

		assert.Equal(t, expectedPubKey, pubKey)

		addr := tc.signerClient.GetPubKey().Address()
		expectedAddr := tc.mockPV.GetPubKey().Address()

		assert.Equal(t, expectedAddr, addr)
	}
}

func TestSignerProposal(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		ts := time.Now()
		want := &types.Proposal{Timestamp: ts}
		have := &types.Proposal{Timestamp: ts}

		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		require.NoError(t, tc.mockPV.SignProposal(tc.chainID, want))
		require.NoError(t, tc.signerClient.SignProposal(tc.chainID, have))

		assert.Equal(t, want.Signature, have.Signature)
	}
}

func TestSignerVote(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		ts := time.Now()
		want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}
		have := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, have))

		assert.Equal(t, want.Signature, have.Signature)
	}
}

func TestSignerVoteResetDeadline(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		ts := time.Now()
		want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}
		have := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		time.Sleep(testTimeoutReadWrite2o3)

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, have))
		assert.Equal(t, want.Signature, have.Signature)

		// TODO(jleni): Clarify what is actually being tested

		// This would exceed the deadline if it was not extended by the previous message
		time.Sleep(testTimeoutReadWrite2o3)

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, have))
		assert.Equal(t, want.Signature, have.Signature)
	}
}

func TestSignerVoteKeepAlive(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		ts := time.Now()
		want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}
		have := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		// Check that even if the client does not request a
		// signature for a long time. The service is still available

		// in this particular case, we use the dialer logger to ensure that
		// test messages are properly interleaved in the test logs
		tc.signerServer.Logger.Debug("TEST: Forced Wait -------------------------------------------------")
		time.Sleep(testTimeoutReadWrite * 3)
		tc.signerServer.Logger.Debug("TEST: Forced Wait DONE---------------------------------------------")

		require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
		require.NoError(t, tc.signerClient.SignVote(tc.chainID, have))

		assert.Equal(t, want.Signature, have.Signature)
	}
}

func TestSignerSignProposalErrors(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		// Replace service with a mock that always fails
		tc.signerServer.privVal = types.NewErroringMockPV()
		tc.mockPV = types.NewErroringMockPV()

		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		ts := time.Now()
		proposal := &types.Proposal{Timestamp: ts}
		err := tc.signerClient.SignProposal(tc.chainID, proposal)
		require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

		err = tc.mockPV.SignProposal(tc.chainID, proposal)
		require.Error(t, err)

		err = tc.signerClient.SignProposal(tc.chainID, proposal)
		require.Error(t, err)
	}
}

func TestSignerSignVoteErrors(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		ts := time.Now()
		vote := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

		// Replace signer service privval with one that always fails
		tc.signerServer.privVal = types.NewErroringMockPV()
		tc.mockPV = types.NewErroringMockPV()

		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		err := tc.signerClient.SignVote(tc.chainID, vote)
		require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

		err = tc.mockPV.SignVote(tc.chainID, vote)
		require.Error(t, err)

		err = tc.signerClient.SignVote(tc.chainID, vote)
		require.Error(t, err)
	}
}

func brokenHandler(privVal types.PrivValidator, request SignerMessage, chainID string) (SignerMessage, error) {
	var res SignerMessage
	var err error

	switch r := request.(type) {

	// This is broken and will answer most requests with a pubkey response
	case *PubKeyRequest:
		res = &PubKeyResponse{nil, nil}
	case *SignVoteRequest:
		res = &PubKeyResponse{nil, nil}
	case *SignProposalRequest:
		res = &PubKeyResponse{nil, nil}

	case *PingRequest:
		err, res = nil, &PingResponse{}

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

		defer tc.signerServer.Stop()
		defer tc.signerClient.Close()

		ts := time.Now()
		want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

		e := tc.signerClient.SignVote(tc.chainID, want)
		assert.EqualError(t, e, "received unexpected response")
	}
}
