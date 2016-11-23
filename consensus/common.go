package consensus

import (
	"github.com/tendermint/tendermint/types"
)

// NOTE: if chanCap=0, this blocks on the event being consumed
func subscribeToEvent(evsw types.EventSwitch, receiver, eventID string, chanCap int) chan interface{} {
	// listen for event
	ch := make(chan interface{}, chanCap)
	types.AddListenerForEvent(evsw, receiver, eventID, func(data types.TMEventData) {
		ch <- data
	})
	return ch
}

// NOTE: this blocks on receiving a response after the event is consumed
func subscribeToEventRespond(evsw types.EventSwitch, receiver, eventID string) chan interface{} {
	// listen for event
	ch := make(chan interface{})
	types.AddListenerForEvent(evsw, receiver, eventID, func(data types.TMEventData) {
		ch <- data
		<-ch
	})
	return ch
}
