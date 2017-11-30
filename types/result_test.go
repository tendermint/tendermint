package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseQuery(t *testing.T) {
	res := ResponseQuery{
		Code:   CodeTypeOK,
		Index:  0,
		Key:    []byte("hello"),
		Value:  []byte("world"),
		Height: 1,
	}
	assert.False(t, res.IsErr())

	res = ResponseQuery{
		Code:   1,
		Index:  0,
		Key:    []byte("hello"),
		Value:  []byte("world"),
		Height: 1,
		Log:    "bad",
	}
	assert.True(t, res.IsErr())
	assert.Equal(t, "Error code (1): bad", res.Error())
}

func TestResponseDeliverTx(t *testing.T) {
	res := ResponseDeliverTx{
		Code: CodeTypeOK,
		Data: []byte("Victor Mancha"),
	}
	assert.False(t, res.IsErr())

	res = ResponseDeliverTx{
		Code: 1,
		Log:  "bad",
	}
	assert.True(t, res.IsErr())
	assert.Equal(t, "Error code (1): bad", res.Error())
}

func TestResponseCheckTx(t *testing.T) {
	res := ResponseCheckTx{
		Code: CodeTypeOK,
		Data: []byte("Talos"),
	}
	assert.False(t, res.IsErr())

	res = ResponseCheckTx{
		Code: 1,
		Log:  "bad",
	}
	assert.True(t, res.IsErr())
	assert.Equal(t, "Error code (1): bad", res.Error())
}

func TestResponseCommit(t *testing.T) {
	res := ResponseCommit{
		Code: CodeTypeOK,
		Data: []byte("Old Lace"),
	}
	assert.False(t, res.IsErr())

	res = ResponseCommit{
		Code: 1,
		Log:  "bad",
	}
	assert.True(t, res.IsErr())
	assert.Equal(t, "Error code (1): bad", res.Error())
}
