package blocksync_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
)

func TestBlockRequest_Validate(t *testing.T) {
	testCases := []struct {
		testName      string
		requestHeight int64
		expectErr     bool
	}{
		{"Valid Request Message", 0, false},
		{"Valid Request Message", 1, false},
		{"Invalid Request Message", -1, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			msg := &bcproto.Message{}
			require.NoError(t, msg.Wrap(&bcproto.BlockRequest{Height: tc.requestHeight}))

			require.Equal(t, tc.expectErr, msg.Validate() != nil)
		})
	}
}

func TestNoBlockResponse_Validate(t *testing.T) {
	testCases := []struct {
		testName          string
		nonResponseHeight int64
		expectErr         bool
	}{
		{"Valid Non-Response Message", 0, false},
		{"Valid Non-Response Message", 1, false},
		{"Invalid Non-Response Message", -1, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			msg := &bcproto.Message{}
			require.NoError(t, msg.Wrap(&bcproto.NoBlockResponse{Height: tc.nonResponseHeight}))

			require.Equal(t, tc.expectErr, msg.Validate() != nil)
		})
	}
}

func TestStatusRequest_Validate(t *testing.T) {
	msg := &bcproto.Message{}
	require.NoError(t, msg.Wrap(&bcproto.StatusRequest{}))
	require.NoError(t, msg.Validate())
}

func TestStatusResponse_Validate(t *testing.T) {
	testCases := []struct {
		testName       string
		responseHeight int64
		expectErr      bool
	}{
		{"Valid Response Message", 0, false},
		{"Valid Response Message", 1, false},
		{"Invalid Response Message", -1, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			msg := &bcproto.Message{}
			require.NoError(t, msg.Wrap(&bcproto.StatusResponse{Height: tc.responseHeight}))

			require.Equal(t, tc.expectErr, msg.Validate() != nil)
		})
	}
}
