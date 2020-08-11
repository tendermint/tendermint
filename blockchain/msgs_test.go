package blockchain

import (
	"math"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	"github.com/tendermint/tendermint/types"
)

func TestBcBlockRequestMessageValidateBasic(t *testing.T) {
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
			request := bcproto.BlockRequest{Height: tc.requestHeight}
			assert.Equal(t, tc.expectErr, ValidateMsg(&request) != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestBcNoBlockResponseMessageValidateBasic(t *testing.T) {
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
			nonResponse := bcproto.NoBlockResponse{Height: tc.nonResponseHeight}
			assert.Equal(t, tc.expectErr, ValidateMsg(&nonResponse) != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestBcStatusRequestMessageValidateBasic(t *testing.T) {
	request := bcproto.StatusRequest{}
	assert.NoError(t, ValidateMsg(&request))
}

func TestBcStatusResponseMessageValidateBasic(t *testing.T) {
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
			response := bcproto.StatusResponse{Height: tc.responseHeight}
			assert.Equal(t, tc.expectErr, ValidateMsg(&response) != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestBlockchainMessageVectors(t *testing.T) {

	block := types.MakeBlock(int64(3), []types.Tx{types.Tx("Hello World")}, nil, nil)

	bpb, err := block.ToProto()
	require.NoError(t, err)

	testCases := []struct {
		testName string
		bmsg     proto.Message
		expBytes []byte
	}{
		{"BlockRequestMessage", &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
			BlockRequest: &bcproto.BlockRequest{Height: 1}}}, []byte{0xa, 0x2, 0x8, 0x1}},
		{"BlockRequestMessage", &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
			BlockRequest: &bcproto.BlockRequest{Height: math.MaxInt64}}},
			[]byte{0xa, 0xa, 0x8, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}},
		{"BlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_BlockResponse{
			BlockResponse: &bcproto.BlockResponse{Block: bpb}}}, []byte{0x1a, 0xb3, 0x1, 0xa,
			0xb0, 0x1, 0xa, 0x59, 0xa, 0x0, 0x18, 0x3, 0x22, 0xb, 0x8, 0x80, 0x92, 0xb8, 0xc3,
			0x98, 0xfe, 0xff, 0xff, 0xff, 0x1, 0x2a, 0x2, 0x12, 0x0, 0x3a, 0x20, 0xc4, 0xda,
			0x88, 0xe8, 0x76, 0x6, 0x2a, 0xa1, 0x54, 0x34, 0x0, 0xd5, 0xd, 0xe, 0xaa, 0xd, 0xac,
			0x88, 0x9, 0x60, 0x57, 0x94, 0x9c, 0xfb, 0x7b, 0xca, 0x7f, 0x3a, 0x48, 0xc0, 0x4b,
			0xf9, 0x6a, 0x20, 0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4,
			0xc8, 0x99, 0x6f, 0xb9, 0x24, 0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4,
			0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55, 0x12, 0x2f, 0xa, 0xb, 0x48, 0x65, 0x6c,
			0x6c, 0x6f, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x12, 0x20, 0xc4, 0xda, 0x88, 0xe8,
			0x76, 0x6, 0x2a, 0xa1, 0x54, 0x34, 0x0, 0xd5, 0xd, 0xe, 0xaa, 0xd, 0xac, 0x88, 0x9,
			0x60, 0x57, 0x94, 0x9c, 0xfb, 0x7b, 0xca, 0x7f, 0x3a, 0x48, 0xc0, 0x4b, 0xf9, 0x1a,
			0x22, 0x12, 0x20, 0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4,
			0xc8, 0x99, 0x6f, 0xb9, 0x24, 0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4,
			0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55}},
		{"NoBlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
			NoBlockResponse: &bcproto.NoBlockResponse{Height: 1}}}, []byte{0x12, 0x2, 0x8, 0x1}},
		{"NoBlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
			NoBlockResponse: &bcproto.NoBlockResponse{Height: math.MaxInt64}}},
			[]byte{0x12, 0xa, 0x8, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}},
		{"StatusRequestMessage", &bcproto.Message{Sum: &bcproto.Message_StatusRequest{
			StatusRequest: &bcproto.StatusRequest{}}},
			[]byte{0x22, 0x0}},
		{"StatusResponseMessage", &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
			StatusResponse: &bcproto.StatusResponse{Height: 1, Base: 2}}},
			[]byte{0x2a, 0x4, 0x8, 0x1, 0x10, 0x2}},
		{"StatusResponseMessage", &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
			StatusResponse: &bcproto.StatusResponse{Height: math.MaxInt64, Base: math.MaxInt64}}},
			[]byte{0x2a, 0x14, 0x8, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f, 0x10,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			bz, _ := proto.Marshal(tc.bmsg)

			require.Equal(t, tc.expBytes, bz)
		})
	}
}
