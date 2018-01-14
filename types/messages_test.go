package types

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	cmn "github.com/tendermint/tmlibs/common"
)

func TestMarshalJSON(t *testing.T) {
	b, err := json.Marshal(&ResponseDeliverTx{})
	assert.Nil(t, err)
	// Do not include empty fields.
	assert.False(t, strings.Contains(string(b), "code"))

	r1 := ResponseCheckTx{
		Code:      1,
		Data:      []byte("hello"),
		GasWanted: 43,
		Fee:       cmn.KI64Pair{[]byte("pho"), 12},
	}
	b, err = json.Marshal(&r1)
	assert.Nil(t, err)

	var r2 ResponseCheckTx
	err = json.Unmarshal(b, &r2)
	assert.Nil(t, err)
	assert.Equal(t, r1, r2)
}

func TestWriteReadMessageSimple(t *testing.T) {
	cases := []proto.Message{
		&RequestEcho{
			Message: "Hello",
		},
	}

	for _, c := range cases {
		buf := new(bytes.Buffer)
		err := WriteMessage(c, buf)
		assert.Nil(t, err)

		msg := new(RequestEcho)
		err = ReadMessage(buf, msg)
		assert.Nil(t, err)

		assert.Equal(t, c, msg)
	}
}

func TestWriteReadMessage(t *testing.T) {
	cases := []proto.Message{
		&Header{
			NumTxs: 4,
		},
		// TODO: add the rest
	}

	for _, c := range cases {
		buf := new(bytes.Buffer)
		err := WriteMessage(c, buf)
		assert.Nil(t, err)

		msg := new(Header)
		err = ReadMessage(buf, msg)
		assert.Nil(t, err)

		assert.Equal(t, c, msg)
	}
}
