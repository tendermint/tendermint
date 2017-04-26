package core_types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
)

func TestStatusIndexer(t *testing.T) {
	assert := assert.New(t)

	var status *ResultStatus
	assert.False(status.TxIndexEnabled())

	status = &ResultStatus{}
	assert.False(status.TxIndexEnabled())

	status.NodeInfo = &p2p.NodeInfo{}
	assert.False(status.TxIndexEnabled())

	cases := []struct {
		expected bool
		other    []string
	}{
		{false, nil},
		{false, []string{}},
		{false, []string{"a=b"}},
		{false, []string{"tx_indexiskv", "some=dood"}},
		{true, []string{"tx_index=on", "tx_index=other"}},
		{true, []string{"^(*^(", "tx_index=on", "a=n=b=d="}},
	}

	for _, tc := range cases {
		status.NodeInfo.Other = tc.other
		assert.Equal(tc.expected, status.TxIndexEnabled())
	}
}
