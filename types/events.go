package types

type EventsMode int8

const (
	EventsModeOff    = EventsMode(0)
	EventsModeCached = EventsMode(1)
	EventsModeOn     = EventsMode(2)
)

type Event struct {
	Key     string
	TxBytes []byte
}
