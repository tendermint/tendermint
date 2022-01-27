package p2p_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
)

func TestNodeAddress_Zero(t *testing.T) {
	peerID := make([]byte, 20)
	a, err := p2p.ParseNodeAddress(fmt.Sprintf("tcp://%x@1.2.3.4:123", peerID))
	assert.NoError(t, err)
	assert.False(t, a.Zero())

	a = p2p.NodeAddress{}
	assert.True(t, a.Zero())
}
