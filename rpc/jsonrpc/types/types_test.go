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

type responseTest struct {
	id       jsonrpcid
	expected string
}

var responseTests = []responseTest{
	{JSONRPCStringID("1"), `"1"`},
	{JSONRPCStringID("alphabet"), `"alphabet"`},
	{JSONRPCStringID(""), `""`},
	{JSONRPCStringID("àáâ"), `"àáâ"`},
	{JSONRPCIntID(-1), "-1"},
	{JSONRPCIntID(0), "0"},
	{JSONRPCIntID(1), "1"},
	{JSONRPCIntID(100), "100"},
}

func TestResponses(t *testing.T) {
	for _, tt := range responseTests {
		req := RPCRequest{
			ID:     tt.id,
			Method: "whatever",
		}

		a := req.MakeResponse(&SampleResult{"hello"})
		b, err := json.Marshal(a)
		require.NoError(t, err)
		s := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"result":{"Value":"hello"}}`, tt.expected)
		assert.Equal(t, s, string(b))

		d := req.MakeErrorf(CodeParseError, "hello world")
		e, err := json.Marshal(d)
		require.NoError(t, err)
		f := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"error":{"code":-32700,"message":"Parse error","data":"hello world"}}`, tt.expected)
		assert.Equal(t, f, string(e))

		g := req.MakeErrorf(CodeMethodNotFound, "foo")
		h, err := json.Marshal(g)
		require.NoError(t, err)
		i := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"error":{"code":-32601,"message":"Method not found","data":"foo"}}`, tt.expected)
		assert.Equal(t, string(h), i)
	}
}

func TestUnmarshallResponses(t *testing.T) {
	for _, tt := range responseTests {
		response := &RPCResponse{}
		err := json.Unmarshal(
			[]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"result":{"Value":"hello"}}`, tt.expected)),
			response,
		)
		require.NoError(t, err)

		req := RPCRequest{ID: tt.id}
		a := req.MakeResponse(&SampleResult{"hello"})
		assert.Equal(t, *response, a)
	}
	response := &RPCResponse{}
	err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":true,"result":{"Value":"hello"}}`), response)
	require.Error(t, err)
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
