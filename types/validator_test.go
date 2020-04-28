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
		expPass1 bool
		expPass2 bool
	}{
		{"success validator", val, true, true},
		{"failure empty", &Validator{}, false, false},
		{"failure nil", nil, false, false},
	}
	for _, tc := range testCases {
		protoVal, err := tc.v1.ToProto()

		if tc.expPass1 {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		val, err := ValidatorFromProto(protoVal)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.Equal(t, tc.v1, val, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}
