package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtoBuf(t *testing.T) {
	testCases := []struct {
		msg     string
		c1      Consensus
		c2      *Consensus
		expPass bool
	}{
		{"sucess empty", Consensus{}, &Consensus{}, true},
		{"success", Consensus{Protocol(1), Protocol(2)}, &Consensus{Protocol(2), Protocol(1)}, true},
		{"nil Consensus 2", Consensus{Protocol(1), Protocol(2)}, nil, false},
	}
	for _, tc := range testCases {
		protoc := tc.c1.ToProto()
		tc.c2.FromProto(protoc)
		if tc.expPass {
			require.Equal(t, &tc.c1, tc.c2, tc.msg)
		} else {
			require.NotEqual(t, &tc.c1, tc.c2, tc.msg)
		}
	}
}
