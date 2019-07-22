package privval

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
)

type signerTestCase struct {
	chainID       string
	mockPV        types.PrivValidator
	signer        *SignerClient
	signerService *SignerDialerEndpoint
}

func getSignerTestCases(t *testing.T) []signerTestCase {
	return getSignerTestCasesCustom(t, true)
}

func getSignerTestCasesCustom(t *testing.T, startDialer bool) []signerTestCase {
	testCases := make([]signerTestCase, 0)

	for _, dtc := range getDialerTestCases(t) {
		chainID := common.RandStr(12)
		mockPV := types.NewMockPV()

		ve, se := getMockEndpoints(t, chainID, mockPV, dtc.addr, dtc.dialer, startDialer)
		sr, err := NewSignerClient(ve)
		require.NoError(t, err)

		tc := signerTestCase{
			chainID:       chainID,
			mockPV:        mockPV,
			signer:        sr,
			signerService: se,
		}

		testCases = append(testCases, tc)
	}

	return testCases
}

func TestSignerClose(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		err := tc.signer.Close()
		assert.NoError(t, err)

		err = tc.signerService.Stop()
		assert.NoError(t, err)
	}
}

func TestSignerGetPubKey(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		func() {
			defer tc.signerService.Stop()
			defer tc.signer.Close()

			pubKey := tc.signer.GetPubKey()
			expectedPubKey := tc.mockPV.GetPubKey()

			assert.Equal(t, expectedPubKey, pubKey)

			addr := tc.signer.GetPubKey().Address()
			expectedAddr := tc.mockPV.GetPubKey().Address()

			assert.Equal(t, expectedAddr, addr)
		}()
	}
}

func TestSignerProposal(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		func() {
			ts := time.Now()
			want := &types.Proposal{Timestamp: ts}
			have := &types.Proposal{Timestamp: ts}

			defer tc.signerService.Stop()
			defer tc.signer.Close()

			require.NoError(t, tc.mockPV.SignProposal(tc.chainID, want))
			require.NoError(t, tc.signer.SignProposal(tc.chainID, have))

			assert.Equal(t, want.Signature, have.Signature)
		}()
	}
}

func TestSignerVote(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		func() {
			ts := time.Now()
			want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}
			have := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

			defer tc.signerService.Stop()
			defer tc.signer.Close()

			require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
			require.NoError(t, tc.signer.SignVote(tc.chainID, have))

			assert.Equal(t, want.Signature, have.Signature)
		}()
	}
}

func TestSignerVoteResetDeadline(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		func() {
			ts := time.Now()
			want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}
			have := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

			defer tc.signerService.Stop()
			defer tc.signer.Close()

			time.Sleep(testTimeoutReadWrite2o3)

			require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
			require.NoError(t, tc.signer.SignVote(tc.chainID, have))
			assert.Equal(t, want.Signature, have.Signature)

			// TODO(jleni): Clarify what is actually being tested

			// This would exceed the deadline if it was not extended by the previous message
			time.Sleep(testTimeoutReadWrite2o3)

			require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
			require.NoError(t, tc.signer.SignVote(tc.chainID, have))
			assert.Equal(t, want.Signature, have.Signature)
		}()
	}
}

func TestSignerVoteKeepAlive(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		func() {
			ts := time.Now()
			want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}
			have := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

			defer tc.signerService.Stop()
			defer tc.signer.Close()

			// Check that even if the client does not request a
			// signature for a long time. The service is will available
			t.Log("TEST. Forced Wait")
			time.Sleep(testTimeoutReadWrite * 2)
			t.Log("TEST. Forced Wait - DONE")

			require.NoError(t, tc.mockPV.SignVote(tc.chainID, want))
			require.NoError(t, tc.signer.SignVote(tc.chainID, have))

			assert.Equal(t, want.Signature, have.Signature)
		}()
	}
}

func TestSignerSignProposalErrors(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		func() {
			// Replace service with a mock that always fails
			tc.signerService.privVal = types.NewErroringMockPV()
			tc.mockPV = types.NewErroringMockPV()

			defer tc.signerService.Stop()
			defer tc.signer.Close()

			ts := time.Now()
			proposal := &types.Proposal{Timestamp: ts}
			err := tc.signer.SignProposal(tc.chainID, proposal)
			require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

			err = tc.mockPV.SignProposal(tc.chainID, proposal)
			require.Error(t, err)

			err = tc.signer.SignProposal(tc.chainID, proposal)
			require.Error(t, err)
		}()
	}
}

func TestSignerSignVoteErrors(t *testing.T) {
	for _, tc := range getSignerTestCases(t) {
		func() {
			ts := time.Now()
			vote := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

			// Replace signer service privval with one that always fails
			tc.signerService.privVal = types.NewErroringMockPV()
			tc.mockPV = types.NewErroringMockPV()

			defer tc.signerService.Stop()
			defer tc.signer.Close()

			err := tc.signer.SignVote(tc.chainID, vote)
			require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

			err = tc.mockPV.SignVote(tc.chainID, vote)
			require.Error(t, err)

			err = tc.signer.SignVote(tc.chainID, vote)
			require.Error(t, err)
		}()
	}
}

type BrokenSignerDialerEndpoint struct {
	*SignerDialerEndpoint
}

func (ss BrokenSignerDialerEndpoint) writeMessage(msg RemoteSignerMsg) (err error) {
	_, err = cdc.MarshalBinaryLengthPrefixedWriter(ss.conn, PubKeyResponse{})
	ss.Logger.Info("Writing bad response!")
	return
}

func TestSignerUnexpectedResponse(t *testing.T) {
	for _, tc := range getSignerTestCasesCustom(t, false) {
		func() {
			tc.signerService.privVal = types.NewMockPV()
			tc.mockPV = types.NewMockPV()

			tmp := BrokenSignerDialerEndpoint{tc.signerService}
			err := tmp.Start()
			require.NoError(t, err)
			defer tmp.Stop()
			defer tc.signer.Close()

			ts := time.Now()
			want := &types.Vote{Timestamp: ts, Type: types.PrecommitType}

			e := tc.signer.SignVote(tc.chainID, want)
			t.Log(e.Error())
			require.Error(t, e)
		}()
	}
}
