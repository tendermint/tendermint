package quorum

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

// TestNewValidator checks if new validator can be created with some validator address, and then the address can be
// looked up using NodeIDResolver
func TestNewValidator(t *testing.T) {
	quorumHash := crypto.RandQuorumHash()
	priv := types.NewMockPVForQuorum(quorumHash)
	pubKey, err := priv.GetPubKey(context.TODO(), quorumHash)
	nodeID := types.NodeIDFromPubKey(pubKey)
	proTxHash := crypto.RandProTxHash()
	require.NoError(t, err)

	validator := types.NewValidator(pubKey, types.DefaultDashVotingPower, proTxHash,
		fmt.Sprintf("tcp://%s@127.0.0.1:12345", nodeID))
	require.NotNil(t, validator)
	newNodeID := validator.NodeAddress.NodeID
	assert.Equal(t, nodeID, newNodeID)

	validator = types.NewValidator(pubKey, types.DefaultDashVotingPower, proTxHash, "127.0.0.1:23456")
	require.NotNil(t, validator)
	assert.EqualValues(t, "127.0.0.1", validator.NodeAddress.Hostname)
	assert.EqualValues(t, 23456, validator.NodeAddress.Port)
	newNodeAddress, err := NewTCPNodeIDResolver().Resolve(validator.NodeAddress)
	assert.Contains(t, err.Error(), "connection refused")
	assert.Zero(t, newNodeAddress)
}
