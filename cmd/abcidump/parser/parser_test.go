package parser

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func decodeHex(t *testing.T, hexadecimal string) []byte {
	ret, err := hex.DecodeString(hexadecimal)
	require.NoError(t, err)
	return ret
}

func TestParser(t *testing.T) {

	testCases := []struct {
		in       []byte
		typeName string
	}{
		{
			in:       decodeHex(t, "4862221220d029205e1750bcadb32f1fece2276054a70f15aeefc55dc1348744769de66caa"),
			typeName: "tendermint.abci.Request",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			in := bytes.NewBuffer(tc.in)
			parser := NewParser(in)
			out := &bytes.Buffer{}
			parser.Out = out

			err := parser.Parse(tc.typeName)
			require.NoError(t, err)
			t.Log(out.String())
		})
	}
}
