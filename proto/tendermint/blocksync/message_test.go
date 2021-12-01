package blocksync_test

import (
	"encoding/hex"
	math "math"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
	"github.com/tendermint/tendermint/types"
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

func TestBlockchainMessageVectors(t *testing.T) {
	block := types.MakeBlock(int64(3), []types.Tx{types.Tx("Hello World")}, nil, nil)
	block.Version.Block = 11 // overwrite updated protocol version

	bpb, err := block.ToProto()
	require.NoError(t, err)

	testCases := []struct {
		testName string
		bmsg     proto.Message
		expBytes string
	}{
		{"BlockRequestMessage", &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
			BlockRequest: &bcproto.BlockRequest{Height: 1}}}, "0a020801"},
		{"BlockRequestMessage", &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
			BlockRequest: &bcproto.BlockRequest{Height: math.MaxInt64}}},
			"0a0a08ffffffffffffffff7f"},
		{"BlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_BlockResponse{
			BlockResponse: &bcproto.BlockResponse{Block: bpb}}}, "1a700a6e0a5b0a02080b1803220b088092b8c398feffffff012a0212003a20c4da88e876062aa1543400d50d0eaa0dac88096057949cfb7bca7f3a48c04bf96a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855120d0a0b48656c6c6f20576f726c641a00"},
		{"NoBlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
			NoBlockResponse: &bcproto.NoBlockResponse{Height: 1}}}, "12020801"},
		{"NoBlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
			NoBlockResponse: &bcproto.NoBlockResponse{Height: math.MaxInt64}}},
			"120a08ffffffffffffffff7f"},
		{"StatusRequestMessage", &bcproto.Message{Sum: &bcproto.Message_StatusRequest{
			StatusRequest: &bcproto.StatusRequest{}}},
			"2200"},
		{"StatusResponseMessage", &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
			StatusResponse: &bcproto.StatusResponse{Height: 1, Base: 2}}},
			"2a0408011002"},
		{"StatusResponseMessage", &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
			StatusResponse: &bcproto.StatusResponse{Height: math.MaxInt64, Base: math.MaxInt64}}},
			"2a1408ffffffffffffffff7f10ffffffffffffffff7f"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			bz, err := proto.Marshal(tc.bmsg)
			require.NoError(t, err)
			require.Equal(t, tc.expBytes, hex.EncodeToString(bz))
		})
	}
}
