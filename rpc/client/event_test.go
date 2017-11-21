package client_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tmlibs/common"

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
	require := require.New(t)
	for i, c := range GetClients() {
		// start for this test it if it wasn't already running
		if !c.IsRunning() {
			// if so, then we start it, listen, and stop it.
			st, err := c.Start()
			require.Nil(err, "%d: %+v", i, err)
			require.True(st, "%d", i)
			defer c.Stop()
		}

		evtTyp := types.EventNewBlockHeader
		evt, err := client.WaitForOneEvent(c, evtTyp, waitForEventTimeout)
		require.Nil(err, "%d: %+v", i, err)
		_, ok := evt.Unwrap().(types.EventDataNewBlockHeader)
		require.True(ok, "%d: %#v", i, evt)
		// TODO: more checks...
	}
}

func TestBlockEvents(t *testing.T) {
	require := require.New(t)
	for i, c := range GetClients() {
		// start for this test it if it wasn't already running
		if !c.IsRunning() {
			// if so, then we start it, listen, and stop it.
			st, err := c.Start()
			require.Nil(err, "%d: %+v", i, err)
			require.True(st, "%d", i)
			defer c.Stop()
		}

		// listen for a new block; ensure height increases by 1
		var firstBlockHeight int
		for j := 0; j < 3; j++ {
			evtTyp := types.EventNewBlock
			evt, err := client.WaitForOneEvent(c, evtTyp, waitForEventTimeout)
			require.Nil(err, "%d: %+v", j, err)
			blockEvent, ok := evt.Unwrap().(types.EventDataNewBlock)
			require.True(ok, "%d: %#v", j, evt)

			block := blockEvent.Block
			if j == 0 {
				firstBlockHeight = block.Header.Height
				continue
			}

			require.Equal(block.Header.Height, firstBlockHeight+j)
		}
	}
}

func TestTxEventsSentWithBroadcastTxAsync(t *testing.T) {
	require := require.New(t)
	for i, c := range GetClients() {
		// start for this test it if it wasn't already running
		if !c.IsRunning() {
			// if so, then we start it, listen, and stop it.
			st, err := c.Start()
			require.Nil(err, "%d: %+v", i, err)
			require.True(st, "%d", i)
			defer c.Stop()
		}

		// make the tx
		_, _, tx := MakeTxKV()
		evtTyp := types.EventTx

		// send async
		txres, err := c.BroadcastTxAsync(tx)
		require.Nil(err, "%+v", err)
		require.True(txres.Code.IsOK())

		// and wait for confirmation
		evt, err := client.WaitForOneEvent(c, evtTyp, waitForEventTimeout)
		require.Nil(err, "%d: %+v", i, err)
		// and make sure it has the proper info
		txe, ok := evt.Unwrap().(types.EventDataTx)
		require.True(ok, "%d: %#v", i, evt)
		// make sure this is the proper tx
		require.EqualValues(tx, txe.Tx)
		require.True(txe.Code.IsOK())
	}
}

func TestTxEventsSentWithBroadcastTxSync(t *testing.T) {
	require := require.New(t)
	for i, c := range GetClients() {
		// start for this test it if it wasn't already running
		if !c.IsRunning() {
			// if so, then we start it, listen, and stop it.
			st, err := c.Start()
			require.Nil(err, "%d: %+v", i, err)
			require.True(st, "%d", i)
			defer c.Stop()
		}

		// make the tx
		_, _, tx := MakeTxKV()
		evtTyp := types.EventTx

		// send sync
		txres, err := c.BroadcastTxSync(tx)
		require.Nil(err, "%+v", err)
		require.True(txres.Code.IsOK())

		// and wait for confirmation
		evt, err := client.WaitForOneEvent(c, evtTyp, waitForEventTimeout)
		require.Nil(err, "%d: %+v", i, err)
		// and make sure it has the proper info
		txe, ok := evt.Unwrap().(types.EventDataTx)
		require.True(ok, "%d: %#v", i, evt)
		// make sure this is the proper tx
		require.EqualValues(tx, txe.Tx)
		require.True(txe.Code.IsOK())
	}
}
