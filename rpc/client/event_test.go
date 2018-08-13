package client_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

var waitForEventTimeout = 5 * time.Second

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := []byte(cmn.RandStr(8))
	v := []byte(cmn.RandStr(8))
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

func TestTxEventsSentWithBroadcastTxAsync(t *testing.T) {
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

			// send async
			txres, err := c.BroadcastTxAsync(tx)
			require.Nil(t, err, "%+v", err)
			require.Equal(t, txres.Code, abci.CodeTypeOK) // FIXME

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

func TestTxEventsSentWithBroadcastTxSync(t *testing.T) {
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

			// send sync
			txres, err := c.BroadcastTxSync(tx)
			require.Nil(t, err, "%+v", err)
			require.Equal(t, txres.Code, abci.CodeTypeOK) // FIXME

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
