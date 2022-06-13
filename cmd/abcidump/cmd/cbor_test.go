package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
)

func TestCborBase64(t *testing.T) {
	var err error

	base := "omdtZXNzYWdleB1DaGFpbkxvY2sgdmVyaWZpY2F0aW9uIGZhaWxlZGRkYXRho2ZoZWlnaHQaJGELXWlibG9" +
		"ja0hhc2h4QDI1MTc3MzgwZDcyMjdjODcyNWFmNDJlMTlkMWFjNDdhYWViMjZkOTM2YjQwMzQ1MDAwMDAxNTI3ZTBmN" +
		"jQ5NzVpc2lnbmF0dXJleMA5YTZmNmUxMWUyMDAwMDAwMTNkZmFiNDZjMmEzMWE2ZGRlZGRiYmNjNzQ3OTMzMzBlODI" +
		"1MTliMTZiNGQyMjYwYWViMDU5MWViMjI3NDAxZjdjMTIxOTU2NWYwMzFlZDg0MzQ0NjVjNjkxZjM4Y2E5MTZhZmI5ZD" +
		"lmYzViZjIwZGM4ODMxMjdhYmI3MWRmNTE1MzI3ZWQzMGIxZTI2Y2I0ZTNlNjRmN2FmNWY0ODJhODBhYzAyODM4NjRkNjY2ZDI="

	cborCmd := &CborCmd{}
	cmd := cborCmd.Command()
	cmd.SetArgs([]string{"decode", "--input", base, "--format", formatBase64})
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
	assert.Contains(t, s, "\"message\": \"ChainLock verification failed\"")
	assert.Contains(t, s, "\"height\": 610339677")
	assert.Contains(t, s, "\"blockHash\": \"25177380d7227c8725af42e19d1ac47aaeb26d936b40345000001527e0f64975\"")
	assert.Contains(t, s, "\"signature\": \"9a6f6e11e200000013dfab46c2a31a6ddeddbbcc74793330e82519b16b4d2260a")
}
