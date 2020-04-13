package blockchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bcproto "github.com/tendermint/tendermint/proto/blockchain"
	"github.com/tendermint/tendermint/types"
)

func TestMsgToProto(t *testing.T) {
	block := types.MakeBlock(1, []types.Tx{[]byte("tx1"), []byte("tx2")}, &types.Commit{}, nil)
	block.Header.ProposerAddress = []byte("12345678901234567890")
	bp, err := block.ToProto()
	require.NoError(t, err)

	tests := []struct {
		name    string
		msg     Message
		want    *bcproto.Message
		wantErr bool
	}{
		{"successful BlockRequest", &BlockRequestMessage{Height: 1},
			&bcproto.Message{
				Sum: &bcproto.Message_BlockRequest{
					BlockRequest: &bcproto.BlockRequest{
						Height: 1,
					},
				},
			}, false},
		{"successful NoBlockResponse", &NoBlockResponseMessage{Height: 1},
			&bcproto.Message{
				Sum: &bcproto.Message_NoBlockResponse{
					NoBlockResponse: &bcproto.NoBlockResponse{
						Height: 1,
					},
				},
			}, false},
		{"successful BlockResponse", &BlockResponseMessage{Block: block},
			&bcproto.Message{
				Sum: &bcproto.Message_BlockResponse{
					BlockResponse: &bcproto.BlockResponse{
						Block: *bp,
					},
				},
			}, false},
		{"successful StatusRequest", &StatusRequestMessage{Height: 100, Base: 1},
			&bcproto.Message{
				Sum: &bcproto.Message_StatusRequest{
					StatusRequest: &bcproto.StatusRequest{
						Height: 100,
						Base:   1,
					},
				},
			}, false},
		{"successful StatusResponse", &StatusResponseMessage{Height: 100, Base: 1},
			&bcproto.Message{
				Sum: &bcproto.Message_StatusResponse{
					StatusResponse: &bcproto.StatusResponse{
						Height: 100,
						Base:   1,
					},
				},
			}, false},
		{"failure", nil, &bcproto.Message{}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pb, err := MsgToProto(tt.msg)
			if tt.wantErr == true {
				assert.Equal(t, err != nil, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.want, pb, tt.name)
		})
	}
}

func TestMsgFromProto(t *testing.T) {
	block := types.MakeBlock(1, []types.Tx{[]byte("tx1"), []byte("tx2")}, &types.Commit{}, nil)
	block.Header.ProposerAddress = []byte("12345678901234567890")
	bp, err := block.ToProto()
	require.NoError(t, err)

	tests := []struct {
		name    string
		want    Message
		pbmsg   bcproto.Message
		wantErr bool
	}{
		{"successful BlockRequest", &BlockRequestMessage{Height: 1},
			bcproto.Message{
				Sum: &bcproto.Message_BlockRequest{
					BlockRequest: &bcproto.BlockRequest{
						Height: 1,
					},
				},
			}, false},
		{"successful NoBlockResponse", &NoBlockResponseMessage{Height: 1},
			bcproto.Message{
				Sum: &bcproto.Message_NoBlockResponse{
					NoBlockResponse: &bcproto.NoBlockResponse{
						Height: 1,
					},
				},
			}, false},
		{"successful BlockResponse", &BlockResponseMessage{Block: block},
			bcproto.Message{
				Sum: &bcproto.Message_BlockResponse{
					BlockResponse: &bcproto.BlockResponse{
						Block: *bp,
					},
				},
			}, false},
		{"successful StatusRequest", &StatusRequestMessage{Height: 100, Base: 1},
			bcproto.Message{
				Sum: &bcproto.Message_StatusRequest{
					StatusRequest: &bcproto.StatusRequest{
						Height: 100,
						Base:   1,
					},
				},
			}, false},
		{"successful StatusResponse", &StatusResponseMessage{Height: 100, Base: 1},
			bcproto.Message{
				Sum: &bcproto.Message_StatusResponse{
					StatusResponse: &bcproto.StatusResponse{
						Height: 100,
						Base:   1,
					},
				},
			}, false},
		{"failure", nil, bcproto.Message{}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg, err := MsgFromProto(tt.pbmsg)
			if tt.wantErr == true {
				assert.Equal(t, err != nil, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.want, msg, tt.name)
		})
	}
}
