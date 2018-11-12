package rpctypes

import (
	"encoding/json"
	"testing"

	"fmt"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/go-amino"
)

type SampleResult struct {
	Value string
}

func TestResponses(t *testing.T) {
	assert := assert.New(t)
	cdc := amino.NewCodec()

	a := NewRPCSuccessResponse(cdc, JSONRPCStringID("1"), &SampleResult{"hello"})
	b, _ := json.Marshal(a)
	s := `{"jsonrpc":"2.0","id":"1","result":{"Value":"hello"}}`
	assert.Equal(string(s), string(b))

	d := RPCParseError(JSONRPCStringID("1"), errors.New("Hello world"))
	e, _ := json.Marshal(d)
	f := `{"jsonrpc":"2.0","id":"1","error":{"code":-32700,"message":"Parse error. Invalid JSON","data":"Hello world"}}`
	assert.Equal(string(f), string(e))

	g := RPCMethodNotFoundError(JSONRPCStringID("2"))
	h, _ := json.Marshal(g)
	i := `{"jsonrpc":"2.0","id":"2","error":{"code":-32601,"message":"Method not found"}}`
	assert.Equal(string(h), string(i))

	j := NewRPCSuccessResponse(cdc, JSONRPCIntID(1), &SampleResult{"hello"})
	k, _ := json.Marshal(j)
	l := `{"jsonrpc":"2.0","id":1,"result":{"Value":"hello"}}`
	assert.Equal(string(l), string(k))
}

func TestUnmarshallResponses(t *testing.T) {
	assert := assert.New(t)
	cdc := amino.NewCodec()
	var err error
	response := &RPCResponse{}
	err = json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":"1","result":{"Value":"hello"}}`), response)
	assert.Nil(err)
	a := NewRPCSuccessResponse(cdc, JSONRPCStringID("1"), &SampleResult{"hello"})
	assert.Equal(*response, a)
	response2 := &RPCResponse{}
	err2 := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":1,"result":{"Value":"hello"}}`), response2)
	assert.Nil(err2)
	j := NewRPCSuccessResponse(cdc, JSONRPCIntID(1), &SampleResult{"hello"})
	assert.Equal(*response2, j)
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
