package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert := assert.New(t)
	for _, tt := range responseTests {
		jsonid := tt.id
		a := NewRPCSuccessResponse(jsonid, &SampleResult{"hello"})
		b, _ := json.Marshal(a)
		s := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"result":{"Value":"hello"}}`, tt.expected)
		assert.Equal(s, string(b))

		d := RPCParseError(errors.New("hello world"))
		e, _ := json.Marshal(d)
		f := `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error. Invalid JSON","data":"hello world"}}`
		assert.Equal(f, string(e))

		g := RPCMethodNotFoundError(jsonid)
		h, _ := json.Marshal(g)
		i := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"error":{"code":-32601,"message":"Method not found"}}`, tt.expected)
		assert.Equal(string(h), i)
	}
}

func TestUnmarshallResponses(t *testing.T) {
	assert := assert.New(t)
	for _, tt := range responseTests {
		response := &RPCResponse{}
		err := json.Unmarshal(
			[]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"result":{"Value":"hello"}}`, tt.expected)),
			response,
		)
		assert.Nil(err)
		a := NewRPCSuccessResponse(tt.id, &SampleResult{"hello"})
		assert.Equal(*response, a)
	}
	response := &RPCResponse{}
	err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":true,"result":{"Value":"hello"}}`), response)
	assert.NotNil(err)
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
