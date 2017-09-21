package consensus

import (
	"github.com/tendermint/tendermint/types"
)

// XXX: WARNING: this function can halt the consensus as firing events is synchronous.
// Make sure to read off the channels, and in the case of subscribeToEventRespond, to write back on it

// NOTE: if chanCap=0, this blocks on the event being consumed
func subscribeToEvent(evsw types.EventSwitch, receiver, eventID string, chanCap int) chan interface{} {
	// listen for event
	ch := make(chan interface{}, chanCap)
	types.AddListenerForEvent(evsw, receiver, eventID, func(data types.TMEventData) {
		ch <- data
	})
	return ch
}
