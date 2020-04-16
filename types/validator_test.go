package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorProtoBuf(t *testing.T) {
	val, _ := RandValidator(true, 100)
	testCases := []struct {
		msg      string
		v1       *Validator
		v2       *Validator
		expPass1 bool
		expPass2 bool
	}{
		{"success empty", val, &Validator{}, true, true},
		{"fail empty", &Validator{}, &Validator{}, false, true},
		{"false nil", nil, &Validator{}, true, true},
		{"false both nil", nil, nil, true, true},
	}
	for _, tc := range testCases {
		protoVal, err := tc.v1.ToProto()

		if tc.expPass1 {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		err = tc.v2.FromProto(protoVal)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.Equal(t, tc.v1, tc.v2, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}
