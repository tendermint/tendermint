package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/types"
)

func TestCanonicalVoteBytesTestVectors(t *testing.T) {

	tests := []struct {
		cvote *tmproto.CanonicalVote
		want  []byte
	}{
		0: {
			&tmproto.CanonicalVote{},
			[]byte{34, 2, 18, 0, 42, 11, 8, 128, 146, 184, 195, 152, 254, 255, 255, 255, 1},
		},
		1: {
			&tmproto.CanonicalVote{Height: 1, Round: 1, Type: tmproto.PrecommitType},
			[]byte{8, 2, 17, 1, 0, 0, 0, 0, 0, 0, 0, 25, 1, 0, 0, 0, 0, 0, 0, 0, 34, 2, 18, 0, 42, 11, 8,
				128, 146, 184, 195, 152, 254, 255, 255, 255, 1},
		},

		//TODO: add more cases
	}
	for i, tc := range tests {
		got, err := tc.cvote.Marshal()
		require.NoError(t, err)
		require.Equal(t, tc.want, got, "test case #%v: got unexpected sign bytes for Vote.", i)
	}
}
