package types

import (
	"bytes"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

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
