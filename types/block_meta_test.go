package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockMetaValidateBasic(t *testing.T) {
	// TODO
}

func TestBlockMeta_ProtoBuf(t *testing.T) {
	tc := []struct {
		msg     string
		bm1     BlockMeta
		bm2     *BlockMeta
		expPass bool
	}{
		{"success empty", BlockMeta{}, &BlockMeta{}, true},
		{"success", BlockMeta{
			BlockID:   BlockID{},
			BlockSize: 1,
			Header:    Header{},
			NumTxs:    2,
		}, &BlockMeta{
			BlockID:   BlockID{},
			BlockSize: 1,
			Header:    Header{},
			NumTxs:    2}, true},
		{"nil bm2", BlockMeta{}, nil, false},
	}
	for _, tt := range tc {
		pb := tt.bm1.ToProto()
		err := tt.bm2.FromProto(pb)
		if tt.expPass {
			require.EqualValues(t, tt.bm1, *tt.bm2, tt.msg)
		} else {
			require.Error(t, err, tt.msg)
		}
	}
}
