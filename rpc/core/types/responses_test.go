package core_types

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
		other    map[string]string
	}{
		{false, nil},
		{false, map[string]string{}},
		{false, map[string]string{"a": "b"}},
		{false, map[string]string{"tx_index": "kv", "some": "dood"}},
		{true, map[string]string{"tx_index": "on"}},
		{true, map[string]string{"^(": "*^(", "tx_index": "on", "a=n": "b=d="}},
	}

	for _, tc := range cases {
		status.NodeInfo.Other = tc.other
		assert.Equal(t, tc.expected, status.TxIndexEnabled())
	}
}
