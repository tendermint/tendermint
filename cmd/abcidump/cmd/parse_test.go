package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/types"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name       string
		input      proto.Message
		args       []string
		conditions []string
	}{
		{
			name:       "Echo",
			input:      types.ToRequestEcho("test echo MESSAGE"),
			args:       []string{"parse", "--type", "tendermint.abci.Request"},
			conditions: []string{"\"message\": \"test echo MESSAGE\""},
		},
		{
			name: "PrepareProposal",
			input: types.ToRequestPrepareProposal(&types.RequestPrepareProposal{
				Height:             1234,
				ProposerProTxHash:  crypto.Checksum([]byte("begin block")),
				NextValidatorsHash: crypto.Checksum([]byte("next validators")),
			}),
			args: []string{"parse"},
			conditions: []string{
				"prepareProposal",
				"\"nextValidatorsHash\": \"79BF/WUzE4YwIPSizbaiGl2m4+JQGSoyy/h8ktnY/lU=\"",
			},
		},
		{
			name:       "response",
			input:      types.ToResponseEcho("some response message"),
			args:       []string{"--type", "tendermint.abci.Response"},
			conditions: []string{"\"message\": \"some response message\""},
		},
	}
	for _, tc := range testCases {
		for _, format := range []string{formatBase64, formatHex} {
			for _, raw := range []bool{true, false} {

				testName := fmt.Sprintf("parse-%s-%s-raw:%t", tc.name, format, raw)
				t.Run(testName, func(t *testing.T) {
					var (
						err error
					)

					input := encodeProtobuf(t, tc.input, format, raw)

					args := append(tc.args, "--"+flagFormat, format, "--"+flagInput, input)
					if raw {
						args = append(args, "--"+flagRaw)
					}

					parseCmd := &ParseCmd{}
					cmd := parseCmd.Command()
					cmd.SetArgs(args)

					outBuf := &bytes.Buffer{}
					cmd.SetOut(outBuf)

					errBuf := &bytes.Buffer{}
					cmd.SetErr(errBuf)
					logger, err = log.NewLogger(log.LogLevelDebug, errBuf)
					require.NoError(t, err)

					err = cmd.Execute()
					assert.NoError(t, err)
					assert.Equal(t, 0, errBuf.Len(), errBuf.String())

					s := outBuf.String()

					for _, condition := range tc.conditions {
						assert.Contains(t, s, condition)
					}

					t.Log(s)
				})
			}
		}
	}
}

func encodeProtobuf(t *testing.T, msg proto.Message, format string, raw bool) string {
	buf := &bytes.Buffer{}
	if raw {
		result, err := proto.Marshal(msg)
		require.NoError(t, err)
		buf.Write(result)
	} else {
		err := types.WriteMessage(msg, buf)
		require.NoError(t, err)
	}

	marshaled := buf.Bytes()

	switch format {
	case formatHex:
		return hex.EncodeToString(marshaled)
	case formatBase64:
		return base64.StdEncoding.EncodeToString(marshaled)
	}

	t.Errorf("invalid format %s", format)
	return ""
}
