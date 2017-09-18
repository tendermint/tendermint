// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		for i := 0; i < 3; i++ {
			evtTyp := types.EventStringNewBlock()
			evt, err := client.WaitForOneEvent(c, evtTyp, 1*time.Second)
			require.Nil(err, "%d: %+v", i, err)
			blockEvent, ok := evt.Unwrap().(types.EventDataNewBlock)
			require.True(ok, "%d: %#v", i, evt)

			block := blockEvent.Block
			if i == 0 {
				firstBlockHeight = block.Header.Height
				continue
			}

			require.Equal(block.Header.Height, firstBlockHeight+i)
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
		_, _, tx := merktest.MakeTxKV()
		evtTyp := types.EventStringTx(types.Tx(tx))

		// send async
		txres, err := c.BroadcastTxSync(tx)
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
