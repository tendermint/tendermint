package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResultQuery(t *testing.T) {
	orig := &ResponseQuery{
		Code:   CodeType_OK,
		Index:  0,
		Key:    []byte("hello"),
		Value:  []byte("world"),
		Height: 1,
	}
	res := orig.Result()
	assert.False(t, res.IsErr())

	orig = &ResponseQuery{
		Code:   CodeType_BadNonce,
		Index:  0,
		Key:    []byte("hello"),
		Value:  []byte("world"),
		Height: 1,
		Log:    "bad",
	}
	res = orig.Result()
	assert.True(t, res.IsErr())
	assert.Equal(t, "Error bad nonce (3): bad", res.Error())
}

func TestResponseDeliverTx(t *testing.T) {
	res := ResponseDeliverTx{
		Code: CodeType_OK,
		Data: []byte("Victor Mancha"),
	}
	assert.False(t, res.IsErr())

	res = ResponseDeliverTx{
		Code: CodeType_InternalError,
		Log:  "bad",
	}
	assert.True(t, res.IsErr())
	assert.Equal(t, "Internal error (1): bad", res.Error())
}

func TestResponseCheckTx(t *testing.T) {
	res := ResponseCheckTx{
		Code: CodeType_OK,
		Data: []byte("Talos"),
	}
	assert.False(t, res.IsErr())

	res = ResponseCheckTx{
		Code: CodeType_InternalError,
		Log:  "bad",
	}
	assert.True(t, res.IsErr())
	assert.Equal(t, "Internal error (1): bad", res.Error())
}

func TestResponseCommit(t *testing.T) {
	res := ResponseCommit{
		Code: CodeType_OK,
		Data: []byte("Old Lace"),
	}
	assert.False(t, res.IsErr())

	res = ResponseCommit{
		Code: CodeType_Unauthorized,
		Log:  "bad",
	}
	assert.True(t, res.IsErr())
	assert.Equal(t, "Unauthorized (4): bad", res.Error())
}
