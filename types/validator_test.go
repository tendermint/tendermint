package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestValidatorValidateBasic(t *testing.T) {
	priv := NewMockPV()
	pubKey, _ := priv.GetPubKey(context.Background())
	testCases := []struct {
		val *Validator
		err bool
		msg string
	}{
		{
			val: NewValidator(pubKey, 1),
			err: false,
			msg: "",
		},
		{
			val: nil,
			err: true,
			msg: "nil validator",
		},
		{
			val: &Validator{
				PubKey: nil,
			},
			err: true,
			msg: "validator does not have a public key",
		},
		{
			val: NewValidator(pubKey, -1),
			err: true,
			msg: "validator has negative voting power",
		},
		{
			val: &Validator{
				PubKey:  pubKey,
				Address: nil,
			},
			err: true,
			msg: "validator address is the wrong size: ",
		},
		{
			val: &Validator{
				PubKey:  pubKey,
				Address: []byte{'a'},
			},
			err: true,
			msg: "validator address is the wrong size: 61",
		},
	}

	for _, tc := range testCases {
		err := tc.val.ValidateBasic()
		if tc.err {
			if assert.Error(t, err) {
				assert.Equal(t, tc.msg, err.Error())
			}
		} else {
			assert.NoError(t, err)
		}
	}
}
