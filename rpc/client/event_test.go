package client_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

var waitForEventTimeout = 5 * time.Second

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := []byte(tmrand.Str(8))
	v := []byte(tmrand.Str(8))
	return k, v, append(k, append([]byte("="), v...)...)
}

func TestHeaderEvents(t *testing.T) {
	for i, c := range GetClients() {
		i, c := i, c // capture params
		t.Run(reflect.TypeOf(c).String(), func(t *testing.T) {
			// start for this test it if it wasn't already running
			if !c.IsRunning() {
				// if so, then we start it, listen, and stop it.
				err := c.Start()
				require.Nil(t, err, "%d: %+v", i, err)
				defer c.Stop()
			}

			evtTyp := types.EventNewBlockHeader
			evt, err := client.WaitForOneEvent(c, evtTyp, waitForEventTimeout)
			require.Nil(t, err, "%d: %+v", i, err)
			_, ok := evt.(types.EventDataNewBlockHeader)
			require.True(t, ok, "%d: %#v", i, evt)
			// TODO: more checks...
		})
	}
}

func TestBlockEvents(t *testing.T) {
	for i, c := range GetClients() {
		i, c := i, c // capture params
		t.Run(reflect.TypeOf(c).String(), func(t *testing.T) {

			// start for this test it if it wasn't already running
			if !c.IsRunning() {
				// if so, then we start it, listen, and stop it.
				err := c.Start()
				require.Nil(t, err, "%d: %+v", i, err)
				defer c.Stop()
			}

			// listen for a new block; ensure height increases by 1
			var firstBlockHeight int64
			for j := 0; j < 3; j++ {
				evtTyp := types.EventNewBlock
				evt, err := client.WaitForOneEvent(c, evtTyp, waitForEventTimeout)
				require.Nil(t, err, "%d: %+v", j, err)
				blockEvent, ok := evt.(types.EventDataNewBlock)
				require.True(t, ok, "%d: %#v", j, evt)

				block := blockEvent.Block
				if j == 0 {
					firstBlockHeight = block.Header.Height
					continue
				}

				require.Equal(t, block.Header.Height, firstBlockHeight+int64(j))
			}
		})
	}
}

func TestTxEventsSentWithBroadcastTxAsync(t *testing.T) { testTxEventsSent(t, "async") }
func TestTxEventsSentWithBroadcastTxSync(t *testing.T)  { testTxEventsSent(t, "sync") }

func testTxEventsSent(t *testing.T, broadcastMethod string) {
	for i, c := range GetClients() {
		i, c := i, c // capture params
		t.Run(reflect.TypeOf(c).String(), func(t *testing.T) {

			// start for this test it if it wasn't already running
			if !c.IsRunning() {
				// if so, then we start it, listen, and stop it.
				err := c.Start()
				require.Nil(t, err, "%d: %+v", i, err)
				defer c.Stop()
			}

			// make the tx
			_, _, tx := MakeTxKV()
			evtTyp := types.EventTx

			// send
			var (
				txres *ctypes.ResultBroadcastTx
				err   error
			)
			switch broadcastMethod {
			case "async":
				txres, err = c.BroadcastTxAsync(tx)
			case "sync":
				txres, err = c.BroadcastTxSync(tx)
			default:
				panic(fmt.Sprintf("Unknown broadcastMethod %s", broadcastMethod))
			}

			require.NoError(t, err)
			require.Equal(t, txres.Code, abci.CodeTypeOK)

			// and wait for confirmation
			evt, err := client.WaitForOneEvent(c, evtTyp, waitForEventTimeout)
			require.Nil(t, err, "%d: %+v", i, err)
			// and make sure it has the proper info
			txe, ok := evt.(types.EventDataTx)
			require.True(t, ok, "%d: %#v", i, evt)
			// make sure this is the proper tx
			require.EqualValues(t, tx, txe.Tx)
			require.True(t, txe.Result.IsOK())
		})
	}
}

// Test HTTPClient resubscribes upon disconnect && subscription error.
// Test Local client resubscribes upon subscription error.
func TestClientsResubscribe(t *testing.T) {
	// TODO(melekes)
}

func TestHTTPReturnsErrorIfClientIsNotRunning(t *testing.T) {
	c := getHTTPClient()

	// on Subscribe
	_, err := c.Subscribe(context.Background(), "TestHeaderEvents",
		types.QueryForEvent(types.EventNewBlockHeader).String())
	assert.Error(t, err)

	// on Unsubscribe
	err = c.Unsubscribe(context.Background(), "TestHeaderEvents",
		types.QueryForEvent(types.EventNewBlockHeader).String())
	assert.Error(t, err)

	// on UnsubscribeAll
	err = c.UnsubscribeAll(context.Background(), "TestHeaderEvents")
	assert.Error(t, err)
}
