package types

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SampleResult struct {
	Value string
}

// Valid JSON identifier texts.
var testIDs = []string{
	`"1"`, `"alphabet"`, `""`, `"àáâ"`, "-1", "0", "1", "100",
}

func TestResponses(t *testing.T) {
	for _, id := range testIDs {
		req := RPCRequest{id: json.RawMessage(id)}

		a := req.MakeResponse(&SampleResult{"hello"})
		b, err := json.Marshal(a)
		require.NoError(t, err, "input id: %q", id)
		s := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"result":{"Value":"hello"}}`, id)
		assert.Equal(t, s, string(b))

		d := req.MakeErrorf(CodeParseError, "hello world")
		e, err := json.Marshal(d)
		require.NoError(t, err)
		f := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"error":{"code":-32700,"message":"Parse error","data":"hello world"}}`, id)
		assert.Equal(t, f, string(e))

		g := req.MakeErrorf(CodeMethodNotFound, "foo")
		h, err := json.Marshal(g)
		require.NoError(t, err)
		i := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"error":{"code":-32601,"message":"Method not found","data":"foo"}}`, id)
		assert.Equal(t, string(h), i)
	}
}

func TestUnmarshallResponses(t *testing.T) {
	for _, id := range testIDs {
		response := &RPCResponse{}
		input := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"result":{"Value":"hello"}}`, id)
		require.NoError(t, json.Unmarshal([]byte(input), &response))

		req := RPCRequest{id: json.RawMessage(id)}
		a := req.MakeResponse(&SampleResult{"hello"})
		assert.Equal(t, *response, a)
	}
	var response RPCResponse
	const input = `{"jsonrpc":"2.0","id":true,"result":{"Value":"hello"}}`
	require.Error(t, json.Unmarshal([]byte(input), &response))
}

func TestRPCError(t *testing.T) {
	assert.Equal(t, "RPC error 12 - Badness: One worse than a code 11",
		fmt.Sprintf("%v", &RPCError{
			Code:    12,
			Message: "Badness",
			Data:    "One worse than a code 11",
		}))

	assert.Equal(t, "RPC error 12 - Badness",
		fmt.Sprintf("%v", &RPCError{
			Code:    12,
			Message: "Badness",
		}))
}
