package block_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/tmhash"
	test "github.com/tendermint/tendermint/internal/test/factory"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/block"
	"github.com/tendermint/tendermint/pkg/meta"
)

func TestBlockMeta_ToProto(t *testing.T) {
	h := test.MakeRandomHeader()
	bi := meta.BlockID{Hash: h.Hash(), PartSetHeader: meta.PartSetHeader{Total: 123, Hash: tmrand.Bytes(tmhash.Size)}}

	bm := &block.BlockMeta{
		BlockID:   bi,
		BlockSize: 200,
		Header:    *h,
		NumTxs:    0,
	}

	tests := []struct {
		testName string
		bm       *block.BlockMeta
		expErr   bool
	}{
		{"success", bm, false},
		{"failure nil", nil, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			pb := tt.bm.ToProto()

			bm, err := block.BlockMetaFromProto(pb)

			if !tt.expErr {
				require.NoError(t, err, tt.testName)
				require.Equal(t, tt.bm, bm, tt.testName)
			} else {
				require.Error(t, err, tt.testName)
			}
		})
	}
}

func TestBlockMeta_ValidateBasic(t *testing.T) {
	h := test.MakeRandomHeader()
	bi := meta.BlockID{Hash: h.Hash(), PartSetHeader: meta.PartSetHeader{Total: 123, Hash: tmrand.Bytes(tmhash.Size)}}
	bi2 := meta.BlockID{Hash: tmrand.Bytes(tmhash.Size),
		PartSetHeader: meta.PartSetHeader{Total: 123, Hash: tmrand.Bytes(tmhash.Size)}}
	bi3 := meta.BlockID{Hash: []byte("incorrect hash"),
		PartSetHeader: meta.PartSetHeader{Total: 123, Hash: []byte("incorrect hash")}}

	bm := &block.BlockMeta{
		BlockID:   bi,
		BlockSize: 200,
		Header:    *h,
		NumTxs:    0,
	}

	bm2 := &block.BlockMeta{
		BlockID:   bi2,
		BlockSize: 200,
		Header:    *h,
		NumTxs:    0,
	}

	bm3 := &block.BlockMeta{
		BlockID:   bi3,
		BlockSize: 200,
		Header:    *h,
		NumTxs:    0,
	}

	tests := []struct {
		name    string
		bm      *block.BlockMeta
		wantErr bool
	}{
		{"success", bm, false},
		{"failure wrong blockID hash", bm2, true},
		{"failure wrong length blockID hash", bm3, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.bm.ValidateBasic(); (err != nil) != tt.wantErr {
				t.Errorf("BlockMeta.ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
