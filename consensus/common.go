package consensus

import (
	"github.com/tendermint/go-events"
)

// NOTE: this is blocking
func subscribeToEvent(evsw *events.EventSwitch, receiver, eventID string, chanCap int) chan interface{} {
	// listen for new round
	ch := make(chan interface{}, chanCap)
	evsw.AddListenerForEvent(receiver, eventID, func(data events.EventData) {
		ch <- data
	})
	return ch
}
