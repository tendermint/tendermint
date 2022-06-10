package parser

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/types"
)

func TestParser(t *testing.T) {

	testCases := []struct {
		in       proto.Message
		typeName string
	}{
		{
			in:       types.ToRequestEcho("Some echo request"),
			typeName: "tendermint.abci.Request",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			proto, err := proto.Marshal(tc.in)
			require.NoError(t, err)
			in := bytes.NewBuffer(proto)
			parser := NewParser(in)
			parser.LengthDelimeted = false

			out := &bytes.Buffer{}
			parser.Out = out

			err = parser.Parse(tc.typeName)
			require.NoError(t, err, proto)
			t.Log(out.String())
		})
	}
}
