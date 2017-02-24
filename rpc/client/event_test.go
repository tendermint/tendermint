package client_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	events "github.com/tendermint/go-events"
	"github.com/tendermint/tendermint/types"
)

func TestEvents(t *testing.T) {
	require := require.New(t)
	for i, c := range GetClients() {
		// test if this client implements event switch as well.
		evsw, ok := c.(types.EventSwitch)
		// TODO: assert this for all clients when it is suported
		// if !assert.True(ok, "%d: %v", i, c) {
		// 	continue
		// }
		if !ok {
			continue
		}

		// start for this test it if it wasn't already running
		if !evsw.IsRunning() {
			// if so, then we start it, listen, and stop it.
			st, err := evsw.Start()
			require.Nil(err, "%d: %+v", i, err)
			require.True(st, "%d", i)
			// defer evsw.Stop()
		}

		// let's wait for the next header...
		listener := "fooz"
		event, timeout := make(chan events.EventData, 1), make(chan bool, 1)
		// start timeout count-down
		go func() {
			time.Sleep(1 * time.Second)
			timeout <- true
		}()

		// register for the next header event
		evsw.AddListenerForEvent(listener, types.EventStringNewBlockHeader(), func(data events.EventData) {
			event <- data
		})
		// make sure to unregister after the test is over
		defer evsw.RemoveListener(listener)

		select {
		case <-timeout:
			require.True(false, "%d: a timeout waiting for event", i)
		case evt := <-event:
			_, ok := evt.(types.EventDataNewBlockHeader)
			require.True(ok, "%d: %#v", i, evt)
		}
	}
}
