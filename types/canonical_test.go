package types

import (
	"reflect"
	"testing"

	"github.com/tendermint/tendermint/crypto"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestCanonicalizeBlockID(t *testing.T) {
	randhash := tmrand.Bytes(crypto.HashSize)
	block1 := tmproto.BlockID{Hash: randhash,
		PartSetHeader: tmproto.PartSetHeader{Total: 5, Hash: randhash}}
	block2 := tmproto.BlockID{Hash: randhash,
		PartSetHeader: tmproto.PartSetHeader{Total: 10, Hash: randhash}}
	cblock1 := tmproto.CanonicalBlockID{Hash: randhash,
		PartSetHeader: tmproto.CanonicalPartSetHeader{Total: 5, Hash: randhash}}
	cblock2 := tmproto.CanonicalBlockID{Hash: randhash,
		PartSetHeader: tmproto.CanonicalPartSetHeader{Total: 10, Hash: randhash}}

	tests := []struct {
		name string
		args tmproto.BlockID
		want *tmproto.CanonicalBlockID
	}{
		{"first", block1, &cblock1},
		{"second", block2, &cblock2},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalizeBlockID(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CanonicalizeBlockID() = %v, want %v", got, tt.want)
			}
		})
	}
}
