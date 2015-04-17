package events

const (
	eventsBufferSize = 1000
)

// An EventCache buffers events for a Fireable
// All events are cached. Filtering happens on Flush
type EventCache struct {
	evsw   Fireable
	events []eventInfo
}

// Create a new EventCache with an EventSwitch as backend
func NewEventCache(evsw Fireable) *EventCache {
	return &EventCache{
		evsw:   evsw,
		events: make([]eventInfo, eventsBufferSize),
	}
}

// a cached event
type eventInfo struct {
	event string
	msg   interface{}
}

// Cache an event to be fired upon finality.
func (evc *EventCache) FireEvent(event string, msg interface{}) {
	// append to list
	evc.events = append(evc.events, eventInfo{event, msg})
}

// Fire events by running evsw.FireEvent on all cached events. Blocks.
// Clears cached events
func (evc *EventCache) Flush() {
	for _, ei := range evc.events {
		evc.evsw.FireEvent(ei.event, ei.msg)
	}
	evc.events = make([]eventInfo, eventsBufferSize)
}
