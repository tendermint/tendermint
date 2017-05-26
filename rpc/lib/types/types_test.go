package rpctypes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type SampleResult struct {
	Value string
}

func TestResponses(t *testing.T) {
	assert := assert.New(t)

	a := NewRPCSuccessResponse("1", &SampleResult{"hello"})
	b, _ := json.Marshal(a)
	s := `{"jsonrpc":"2.0","id":"1","result":{"Value":"hello"}}`
	assert.Equal(string(s), string(b))

	d := RPCParseError("1")
	e, _ := json.Marshal(d)
	f := `{"jsonrpc":"2.0","id":"1","error":{"code":-32700,"message":"Parse error. Invalid JSON"}}`
	assert.Equal(string(f), string(e))

	g := RPCMethodNotFoundError("2")
	h, _ := json.Marshal(g)
	i := `{"jsonrpc":"2.0","id":"2","error":{"code":-32601,"message":"Method not found"}}`
	assert.Equal(string(h), string(i))

}
