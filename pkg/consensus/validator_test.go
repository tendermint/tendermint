package consensus_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	test "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/pkg/consensus"
)

func TestValidatorProtoBuf(t *testing.T) {
	val, _ := test.RandValidator(true, 100)
	testCases := []struct {
		msg      string
		v1       *consensus.Validator
		expPass1 bool
		expPass2 bool
	}{
		{"success validator", val, true, true},
		{"failure empty", &consensus.Validator{}, false, false},
		{"failure nil", nil, false, false},
	}
	for _, tc := range testCases {
		protoVal, err := tc.v1.ToProto()

		if tc.expPass1 {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		val, err := consensus.ValidatorFromProto(protoVal)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.Equal(t, tc.v1, val, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

func TestValidatorValidateBasic(t *testing.T) {
	priv := consensus.NewMockPV()
	pubKey, _ := priv.GetPubKey(context.Background())
	testCases := []struct {
		val *consensus.Validator
		err bool
		msg string
	}{
		{
			val: consensus.NewValidator(pubKey, 1),
			err: false,
			msg: "",
		},
		{
			val: nil,
			err: true,
			msg: "nil validator",
		},
		{
			val: &consensus.Validator{
				PubKey: nil,
			},
			err: true,
			msg: "validator does not have a public key",
		},
		{
			val: consensus.NewValidator(pubKey, -1),
			err: true,
			msg: "validator has negative voting power",
		},
		{
			val: &consensus.Validator{
				PubKey:  pubKey,
				Address: nil,
			},
			err: true,
			msg: "validator address is the wrong size: ",
		},
		{
			val: &consensus.Validator{
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
