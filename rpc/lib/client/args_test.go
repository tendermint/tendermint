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

package rpcclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Tx []byte

type Foo struct {
	Bar int
	Baz string
}

func TestArgToJSON(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	cases := []struct {
		input    interface{}
		expected string
	}{
		{[]byte("1234"), "0x31323334"},
		{Tx("654"), "0x363534"},
		{Foo{7, "hello"}, `{"Bar":7,"Baz":"hello"}`},
	}

	for i, tc := range cases {
		args := map[string]interface{}{"data": tc.input}
		err := argsToJson(args)
		require.Nil(err, "%d: %+v", i, err)
		require.Equal(1, len(args), "%d", i)
		data, ok := args["data"].(string)
		require.True(ok, "%d: %#v", i, args["data"])
		assert.Equal(tc.expected, data, "%d", i)
	}
}
