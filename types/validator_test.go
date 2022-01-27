package types

import (
	"fmt"
	"testing"

	"github.com/tendermint/tendermint/crypto"
	dashtypes "github.com/tendermint/tendermint/dash/types"
	"github.com/tendermint/tendermint/p2p"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatorProtoBuf(t *testing.T) {
	val, _ := RandValidator()
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
	quorumHash := crypto.RandQuorumHash()
	priv := NewMockPVForQuorum(quorumHash)
	pubKey, err := priv.GetPubKey(quorumHash)
	require.NoError(t, err)

	testCases := []struct {
		val *Validator
		err bool
		msg string
	}{
		{
			val: NewValidatorDefaultVotingPower(pubKey, priv.ProTxHash),
			err: false,
			msg: "no error",
		},
		{
			val: nil,
			err: true,
			msg: "nil validator",
		},
		{
			val: &Validator{
				PubKey:      nil,
				ProTxHash:   crypto.RandProTxHash(),
				VotingPower: 100,
			},
			err: false,
			msg: "no error",
		},
		{
			val: NewValidator(pubKey, -1, priv.ProTxHash, ""),
			err: true,
			msg: "validator has negative voting power",
		},
		{
			val: &Validator{
				PubKey:    pubKey,
				ProTxHash: crypto.CRandBytes(12),
			},
			err: true,
			msg: "validator proTxHash is the wrong size: 12",
		},
		{
			val: &Validator{
				PubKey:    pubKey,
				ProTxHash: nil,
			},
			err: true,
			msg: "validator does not have a provider transaction hash",
		},
	}

	for _, tc := range testCases {
		tcRun := tc
		t.Run(tc.msg, func(t *testing.T) {
			err := tcRun.val.ValidateBasic()
			if tcRun.err {
				if assert.Error(t, err) {
					assert.Equal(t, tcRun.msg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewValidator(t *testing.T) {
	quorumHash := crypto.RandQuorumHash()
	priv := NewMockPVForQuorum(quorumHash)
	pubKey, err := priv.GetPubKey(quorumHash)
	nodeID := p2p.PubKeyToID(pubKey)
	proTxHash := crypto.RandProTxHash()
	require.NoError(t, err)

	validator := NewValidator(pubKey, DefaultDashVotingPower, proTxHash,
		fmt.Sprintf("tcp://%s@127.0.0.1:12345", nodeID))
	require.NotNil(t, validator)
	newNodeID := validator.NodeAddress.NodeID
	assert.Equal(t, nodeID, newNodeID)

	validator = NewValidator(pubKey, DefaultDashVotingPower, proTxHash, "127.0.0.1:23456")
	require.NotNil(t, validator)
	assert.EqualValues(t, "127.0.0.1", validator.NodeAddress.Hostname)
	assert.EqualValues(t, 23456, validator.NodeAddress.Port)
	assert.EqualValues(t, "tcp", validator.NodeAddress.Protocol)
	newNodeID, err = dashtypes.NewTCPNodeIDResolver().Resolve(validator.NodeAddress)
	assert.Contains(t, err.Error(), "connection refused")
	assert.Zero(t, newNodeID)
}
