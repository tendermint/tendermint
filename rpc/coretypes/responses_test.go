package coretypes

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/types"
)

func TestStatusIndexer(t *testing.T) {
	var status *ResultStatus
	assert.False(t, status.TxIndexEnabled())

	status = &ResultStatus{}
	assert.False(t, status.TxIndexEnabled())

	status.NodeInfo = types.NodeInfo{}
	assert.False(t, status.TxIndexEnabled())

	cases := []struct {
		expected bool
		other    types.NodeInfoOther
	}{
		{false, types.NodeInfoOther{}},
		{false, types.NodeInfoOther{TxIndex: "aa"}},
		{false, types.NodeInfoOther{TxIndex: "off"}},
		{true, types.NodeInfoOther{TxIndex: "on"}},
	}

	for _, tc := range cases {
		status.NodeInfo.Other = tc.other
		assert.Equal(t, tc.expected, status.TxIndexEnabled())
	}
}
