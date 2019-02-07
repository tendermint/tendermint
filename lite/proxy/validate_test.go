package proxy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/lite/proxy"
	"github.com/tendermint/tendermint/types"
)

var (
	deadBeefTxs  = types.Txs{[]byte("DE"), []byte("AD"), []byte("BE"), []byte("EF")}
	deadBeefHash = deadBeefTxs.Hash()
	testTime1    = time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC)
	testTime2    = time.Date(2017, 1, 2, 1, 1, 1, 1, time.UTC)
)

var hdrHeight11 = types.Header{
	Height:         11,
	Time:           testTime1,
	ValidatorsHash: []byte("Tendermint"),
}

func TestValidateBlock(t *testing.T) {
	tests := []struct {
		block        *types.Block
		signedHeader types.SignedHeader
		wantErr      string
	}{
		{
			block: nil, wantErr: "non-nil Block",
		},
		{
			block: &types.Block{}, wantErr: "unexpected empty SignedHeader",
		},

		// Start Header.Height mismatch test
		{
			block:        &types.Block{Header: types.Header{Height: 10}},
			signedHeader: types.SignedHeader{Header: &types.Header{Height: 11}},
			wantErr:      "Header heights mismatched",
		},

		{
			block:        &types.Block{Header: types.Header{Height: 11}},
			signedHeader: types.SignedHeader{Header: &types.Header{Height: 11}},
		},
		// End Header.Height mismatch test

		// Start Header.Hash mismatch test
		{
			block:        &types.Block{Header: hdrHeight11},
			signedHeader: types.SignedHeader{Header: &types.Header{Height: 11}},
			wantErr:      "Headers don't match",
		},

		{
			block:        &types.Block{Header: hdrHeight11},
			signedHeader: types.SignedHeader{Header: &hdrHeight11},
		},
		// End Header.Hash mismatch test

		// Start Header.Data hash mismatch test
		{
			block: &types.Block{
				Header: types.Header{Height: 11},
				Data:   types.Data{Txs: []types.Tx{[]byte("0xDE"), []byte("AD")}},
			},
			signedHeader: types.SignedHeader{
				Header: &types.Header{Height: 11},
				Commit: types.NewCommit(types.BlockID{Hash: []byte("0xDEADBEEF")}, nil),
			},
			wantErr: "Data hash doesn't match header",
		},
		{
			block: &types.Block{
				Header: types.Header{Height: 11, DataHash: deadBeefHash},
				Data:   types.Data{Txs: deadBeefTxs},
			},
			signedHeader: types.SignedHeader{
				Header: &types.Header{Height: 11},
				Commit: types.NewCommit(types.BlockID{Hash: []byte("DEADBEEF")}, nil),
			},
		},
		// End Header.Data hash mismatch test
	}

	for i, tt := range tests {
		err := proxy.ValidateBlock(tt.block, tt.signedHeader)
		if tt.wantErr != "" {
			if err == nil {
				assert.FailNowf(t, "Unexpectedly passed", "#%d", i)
			} else {
				assert.Contains(t, err.Error(), tt.wantErr, "#%d should contain the substring\n\n", i)
			}
			continue
		}

		assert.Nil(t, err, "#%d: expecting a nil error", i)
	}
}

func TestValidateBlockMeta(t *testing.T) {
	tests := []struct {
		meta         *types.BlockMeta
		signedHeader types.SignedHeader
		wantErr      string
	}{
		{
			meta: nil, wantErr: "non-nil BlockMeta",
		},
		{
			meta: &types.BlockMeta{}, wantErr: "unexpected empty SignedHeader",
		},

		// Start Header.Height mismatch test
		{
			meta:         &types.BlockMeta{Header: types.Header{Height: 10}},
			signedHeader: types.SignedHeader{Header: &types.Header{Height: 11}},
			wantErr:      "Header heights mismatched",
		},

		{
			meta:         &types.BlockMeta{Header: types.Header{Height: 11}},
			signedHeader: types.SignedHeader{Header: &types.Header{Height: 11}},
		},
		// End Header.Height mismatch test

		// Start Headers don't match test
		{
			meta:         &types.BlockMeta{Header: hdrHeight11},
			signedHeader: types.SignedHeader{Header: &types.Header{Height: 11}},
			wantErr:      "Headers don't match",
		},

		{
			meta:         &types.BlockMeta{Header: hdrHeight11},
			signedHeader: types.SignedHeader{Header: &hdrHeight11},
		},

		{
			meta: &types.BlockMeta{
				Header: types.Header{
					Height:         11,
					ValidatorsHash: []byte("lite-test"),
					// TODO: should be able to use empty time after Amino upgrade
					Time: testTime1,
				},
			},
			signedHeader: types.SignedHeader{
				Header: &types.Header{Height: 11, DataHash: deadBeefHash},
			},
			wantErr: "Headers don't match",
		},

		{
			meta: &types.BlockMeta{
				Header: types.Header{
					Height: 11, DataHash: deadBeefHash,
					ValidatorsHash: []byte("Tendermint"),
					Time:           testTime1,
				},
			},
			signedHeader: types.SignedHeader{
				Header: &types.Header{
					Height: 11, DataHash: deadBeefHash,
					ValidatorsHash: []byte("Tendermint"),
					Time:           testTime2,
				},
				Commit: types.NewCommit(types.BlockID{Hash: []byte("DEADBEEF")}, nil),
			},
			wantErr: "Headers don't match",
		},

		{
			meta: &types.BlockMeta{
				Header: types.Header{
					Height: 11, DataHash: deadBeefHash,
					ValidatorsHash: []byte("Tendermint"),
					Time:           testTime2,
				},
			},
			signedHeader: types.SignedHeader{
				Header: &types.Header{
					Height: 11, DataHash: deadBeefHash,
					ValidatorsHash: []byte("Tendermint-x"),
					Time:           testTime2,
				},
				Commit: types.NewCommit(types.BlockID{Hash: []byte("DEADBEEF")}, nil),
			},
			wantErr: "Headers don't match",
		},
		// End Headers don't match test
	}

	for i, tt := range tests {
		err := proxy.ValidateBlockMeta(tt.meta, tt.signedHeader)
		if tt.wantErr != "" {
			if err == nil {
				assert.FailNowf(t, "Unexpectedly passed", "#%d: wanted error %q", i, tt.wantErr)
			} else {
				assert.Contains(t, err.Error(), tt.wantErr, "#%d should contain the substring\n\n", i)
			}
			continue
		}

		assert.Nil(t, err, "#%d: expecting a nil error", i)
	}
}
