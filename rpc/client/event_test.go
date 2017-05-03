package client_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	merktest "github.com/tendermint/merkleeyes/testutil"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

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

		evtTyp := types.EventStringNewBlockHeader()
		evt, err := client.WaitForOneEvent(c, evtTyp, 1*time.Second)
		require.Nil(err, "%d: %+v", i, err)
		_, ok := evt.Unwrap().(types.EventDataNewBlockHeader)
		require.True(ok, "%d: %#v", i, evt)
		// TODO: more checks...
	}
}

func TestTxEvents(t *testing.T) {
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
		_, _, tx := merktest.MakeTxKV()
		evtTyp := types.EventStringTx(types.Tx(tx))

		// send async
		txres, err := c.BroadcastTxAsync(tx)
		require.Nil(err, "%+v", err)
		require.True(txres.Code.IsOK())

		// and wait for confirmation
		evt, err := client.WaitForOneEvent(c, evtTyp, 1*time.Second)
		require.Nil(err, "%d: %+v", i, err)
		// and make sure it has the proper info
		txe, ok := evt.Unwrap().(types.EventDataTx)
		require.True(ok, "%d: %#v", i, evt)
		// make sure this is the proper tx
		require.EqualValues(tx, txe.Tx)
		require.True(txe.Code.IsOK())
	}
}
