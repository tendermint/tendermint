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

var waitForEventTimeout = 8 * time.Second

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := []byte(tmrand.Str(8))
	v := []byte(tmrand.Str(8))
	return k, v, append(k, append([]byte("="), v...)...)
}

func TestHeaderEvents(t *testing.T) {
	for i, c := range GetClients() {
		i, c := i, c
		t.Run(reflect.TypeOf(c).String(), func(t *testing.T) {
			// start for this test it if it wasn't already running
			if !c.IsRunning() {
				// if so, then we start it, listen, and stop it.
				err := c.Start()
				require.Nil(t, err, "%d: %+v", i, err)
				t.Cleanup(func() {
					if err := c.Stop(); err != nil {
						t.Error(err)
					}
				})
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

// subscribe to new blocks and make sure height increments by 1
func TestBlockEvents(t *testing.T) {
	for _, c := range GetClients() {
		c := c
		t.Run(reflect.TypeOf(c).String(), func(t *testing.T) {

			// start for this test it if it wasn't already running
			if !c.IsRunning() {
				// if so, then we start it, listen, and stop it.
				err := c.Start()
				require.Nil(t, err)
				t.Cleanup(func() {
					if err := c.Stop(); err != nil {
						t.Error(err)
					}
				})
			}

			const subscriber = "TestBlockEvents"

			eventCh, err := c.Subscribe(context.Background(), subscriber, types.QueryForEvent(types.EventNewBlock).String())
			require.NoError(t, err)
			t.Cleanup(func() {
				if err := c.UnsubscribeAll(context.Background(), subscriber); err != nil {
					t.Error(err)
				}
			})

			var firstBlockHeight int64
			for i := int64(0); i < 3; i++ {
				event := <-eventCh
				blockEvent, ok := event.Data.(types.EventDataNewBlock)
				require.True(t, ok)

				block := blockEvent.Block

				if firstBlockHeight == 0 {
					firstBlockHeight = block.Header.Height
				}

				require.Equal(t, firstBlockHeight+i, block.Header.Height)
			}
		})
	}
}

func TestTxEventsSentWithBroadcastTxAsync(t *testing.T) { testTxEventsSent(t, "async") }
func TestTxEventsSentWithBroadcastTxSync(t *testing.T)  { testTxEventsSent(t, "sync") }

func testTxEventsSent(t *testing.T, broadcastMethod string) {
	for _, c := range GetClients() {
		c := c
		t.Run(reflect.TypeOf(c).String(), func(t *testing.T) {

			// start for this test it if it wasn't already running
			if !c.IsRunning() {
				// if so, then we start it, listen, and stop it.
				err := c.Start()
				require.Nil(t, err)
				t.Cleanup(func() {
					if err := c.Stop(); err != nil {
						t.Error(err)
					}
				})
			}

			// make the tx
			_, _, tx := MakeTxKV()

			// send
			go func() {
				var (
					txres *ctypes.ResultBroadcastTx
					err   error
					ctx   = context.Background()
				)
				switch broadcastMethod {
				case "async":
					txres, err = c.BroadcastTxAsync(ctx, tx)
				case "sync":
					txres, err = c.BroadcastTxSync(ctx, tx)
				default:
					panic(fmt.Sprintf("Unknown broadcastMethod %s", broadcastMethod))
				}
				if assert.NoError(t, err) {
					assert.Equal(t, txres.Code, abci.CodeTypeOK)
				}
			}()

			// and wait for confirmation
			evt, err := client.WaitForOneEvent(c, types.EventTx, waitForEventTimeout)
			require.Nil(t, err)

			// and make sure it has the proper info
			txe, ok := evt.(types.EventDataTx)
			require.True(t, ok)

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
