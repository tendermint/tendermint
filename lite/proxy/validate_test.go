package proxy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/lite"
	"github.com/tendermint/tendermint/lite/proxy"
	"github.com/tendermint/tendermint/types"
)

var (
	deadBeefTxs = types.Txs{[]byte("DE"), []byte("AD"), []byte("BE"), []byte("EF")}

	deadBeefRipEmd160Hash = deadBeefTxs.Hash()
)

var hdrHeight11Tendermint = &types.Header{
	Height:         11,
	Time:           time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC),
	ValidatorsHash: []byte("Tendermint"),
}

func TestValidateBlock(t *testing.T) {
	tests := []struct {
		block   *types.Block
		commit  lite.Commit
		wantErr string
	}{
		{
			block: nil, wantErr: "non-nil Block",
		},
		{
			block: &types.Block{}, wantErr: "nil Header",
		},
		{
			block: &types.Block{Header: new(types.Header)},
		},

		// Start Header.Height mismatch test
		{
			block:   &types.Block{Header: &types.Header{Height: 10}},
			commit:  lite.Commit{Header: &types.Header{Height: 11}},
			wantErr: "don't match - 10 vs 11",
		},

		{
			block:  &types.Block{Header: &types.Header{Height: 11}},
			commit: lite.Commit{Header: &types.Header{Height: 11}},
		},
		// End Header.Height mismatch test

		// Start Header.Hash mismatch test
		{
			block:   &types.Block{Header: hdrHeight11Tendermint},
			commit:  lite.Commit{Header: &types.Header{Height: 11}},
			wantErr: "Headers don't match",
		},

		{
			block:  &types.Block{Header: hdrHeight11Tendermint},
			commit: lite.Commit{Header: hdrHeight11Tendermint},
		},
		// End Header.Hash mismatch test

		// Start Header.Data hash mismatch test
		{
			block: &types.Block{
				Header: &types.Header{Height: 11},
				Data:   &types.Data{Txs: []types.Tx{[]byte("0xDE"), []byte("AD")}},
			},
			commit: lite.Commit{
				Header: &types.Header{Height: 11},
				Commit: &types.Commit{BlockID: types.BlockID{Hash: []byte("0xDEADBEEF")}},
			},
			wantErr: "Data hash doesn't match header",
		},
		{
			block: &types.Block{
				Header: &types.Header{Height: 11, DataHash: deadBeefRipEmd160Hash},
				Data:   &types.Data{Txs: deadBeefTxs},
			},
			commit: lite.Commit{
				Header: &types.Header{Height: 11},
				Commit: &types.Commit{BlockID: types.BlockID{Hash: []byte("DEADBEEF")}},
			},
		},
		// End Header.Data hash mismatch test
	}

	for i, tt := range tests {
		err := proxy.ValidateBlock(tt.block, tt.commit)
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
		meta    *types.BlockMeta
		commit  lite.Commit
		wantErr string
	}{
		{
			meta: nil, wantErr: "non-nil BlockMeta",
		},
		{
			meta: &types.BlockMeta{}, wantErr: "non-nil Header",
		},
		{
			meta: &types.BlockMeta{Header: new(types.Header)},
		},

		// Start Header.Height mismatch test
		{
			meta:    &types.BlockMeta{Header: &types.Header{Height: 10}},
			commit:  lite.Commit{Header: &types.Header{Height: 11}},
			wantErr: "don't match - 10 vs 11",
		},

		{
			meta:   &types.BlockMeta{Header: &types.Header{Height: 11}},
			commit: lite.Commit{Header: &types.Header{Height: 11}},
		},
		// End Header.Height mismatch test

		// Start Headers don't match test
		{
			meta:    &types.BlockMeta{Header: hdrHeight11Tendermint},
			commit:  lite.Commit{Header: &types.Header{Height: 11}},
			wantErr: "Headers don't match",
		},

		{
			meta:   &types.BlockMeta{Header: hdrHeight11Tendermint},
			commit: lite.Commit{Header: hdrHeight11Tendermint},
		},

		{
			meta: &types.BlockMeta{
				Header: &types.Header{
					Height: 11,
					// TODO: (@odeke-em) inquire why ValidatorsHash has to be non-blank
					// for the Header to be hashed. Perhaps this is a security hole because
					// an aggressor could perhaps pass in headers that don't have
					// ValidatorsHash set and we won't be able to validate blocks.
					ValidatorsHash: []byte("lite-test"),
					// TODO: (@odeke-em) file an issue with Tendermint to get them to update
					// to the latest go-wire, then no more need for this value fill to avoid
					// the time zero value of less than 1970.
					Time: time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC),
				},
			},
			commit: lite.Commit{
				Header: &types.Header{Height: 11, DataHash: deadBeefRipEmd160Hash},
			},
			wantErr: "Headers don't match",
		},

		{
			meta: &types.BlockMeta{
				Header: &types.Header{
					Height: 11, DataHash: deadBeefRipEmd160Hash,
					ValidatorsHash: []byte("Tendermint"),
					Time:           time.Date(2017, 1, 2, 1, 1, 1, 1, time.UTC),
				},
			},
			commit: lite.Commit{
				Header: &types.Header{
					Height: 11, DataHash: deadBeefRipEmd160Hash,
					ValidatorsHash: []byte("Tendermint"),
					Time:           time.Date(2018, 1, 2, 1, 1, 1, 1, time.UTC),
				},
				Commit: &types.Commit{BlockID: types.BlockID{Hash: []byte("DEADBEEF")}},
			},
			wantErr: "Headers don't match",
		},

		{
			meta: &types.BlockMeta{
				Header: &types.Header{
					Height: 11, DataHash: deadBeefRipEmd160Hash,
					ValidatorsHash: []byte("Tendermint"),
					Time:           time.Date(2017, 1, 2, 1, 1, 1, 1, time.UTC),
				},
			},
			commit: lite.Commit{
				Header: &types.Header{
					Height: 11, DataHash: deadBeefRipEmd160Hash,
					ValidatorsHash: []byte("Tendermint-x"),
					Time:           time.Date(2017, 1, 2, 1, 1, 1, 1, time.UTC),
				},
				Commit: &types.Commit{BlockID: types.BlockID{Hash: []byte("DEADBEEF")}},
			},
			wantErr: "Headers don't match",
		},
		// End Headers don't match test
	}

	for i, tt := range tests {
		err := proxy.ValidateBlockMeta(tt.meta, tt.commit)
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
