package util

import (
	"fmt"
	"os"
	"os/signal"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

// WaitForInterrupt blocks until a SIGINT is received by the process.
func WaitForInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
}

// VerifyNotImplemented calls f and verifies that it returns both a
// false-valued flag and a non-nil error whose string matching the expected
// "not supported" message with label prefixed.
func VerifyNotImplemented(t *testing.T, label string, sinkType string, f func() (bool, error)) {
	t.Helper()
	t.Logf("Verifying that %q reports it is not implemented", label)

	want := label + fmt.Sprintf(" is not supported via the %s event sink", sinkType)
	ok, err := f()
	assert.False(t, ok)
	require.NotNil(t, err)
	assert.Equal(t, want, err.Error())
}

// TxResultWithEvents constructs a fresh transaction result with fixed values
// for testing, that includes the specified events.
func TxResultWithEvents(events []abci.Event) *abci.TxResult {
	return &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     types.Tx("HELLO WORLD"),
		Result: abci.ResponseDeliverTx{
			Data:   []byte{0},
			Code:   abci.CodeTypeOK,
			Log:    "",
			Events: events,
		},
	}
}
