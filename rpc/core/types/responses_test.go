package coretypes

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/p2p"
)

func TestStatusIndexer(t *testing.T) {
	var status *ResultStatus
	assert.False(t, status.TxIndexEnabled())

	status = &ResultStatus{}
	assert.False(t, status.TxIndexEnabled())

	status.NodeInfo = p2p.NodeInfo{}
	assert.False(t, status.TxIndexEnabled())

	cases := []struct {
		expected bool
		other    p2p.NodeInfoOther
	}{
		{false, p2p.NodeInfoOther{}},
		{false, p2p.NodeInfoOther{TxIndex: "aa"}},
		{false, p2p.NodeInfoOther{TxIndex: "off"}},
		{true, p2p.NodeInfoOther{TxIndex: "on"}},
	}

	for _, tc := range cases {
		status.NodeInfo.Other = tc.other
		assert.Equal(t, tc.expected, status.TxIndexEnabled())
	}
}
