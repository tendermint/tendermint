package client_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

const waitForEventTimeout = 2 * time.Second

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := []byte(tmrand.Str(8))
	v := []byte(tmrand.Str(8))
	return k, v, append(k, append([]byte("="), v...)...)
}

func testTxEventsSent(ctx context.Context, t *testing.T, broadcastMethod string, c client.Client) {
	t.Helper()
	// make the tx
	_, _, tx := MakeTxKV()

	// send
	done := make(chan struct{})
	go func() {
		defer close(done)
		var (
			txres *coretypes.ResultBroadcastTx
			err   error
		)
		switch broadcastMethod {
		case "async":
			txres, err = c.BroadcastTxAsync(ctx, tx)
		case "sync":
			txres, err = c.BroadcastTxSync(ctx, tx)
		default:
			require.FailNowf(t, "Unknown broadcastMethod %s", broadcastMethod)
		}
		if assert.NoError(t, err) {
			assert.Equal(t, txres.Code, abci.CodeTypeOK)
		}
	}()

	// and wait for confirmation
	ectx, cancel := context.WithTimeout(ctx, waitForEventTimeout)
	defer cancel()

	// Wait for the transaction we sent to be confirmed.
	query := fmt.Sprintf(`tm.event = '%s' AND tx.hash = '%X'`,
		types.EventTxValue, types.Tx(tx).Hash())
	evt, err := client.WaitForOneEvent(ectx, c, query)
	require.NoError(t, err)

	// and make sure it has the proper info
	txe, ok := evt.(types.EventDataTx)
	require.True(t, ok)

	// make sure this is the proper tx
	require.EqualValues(t, tx, txe.Tx)
	require.True(t, txe.Result.IsOK())
	<-done
}
