// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
