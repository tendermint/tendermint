package types

type EventsMode int8

const (
	EventsModeOff = EventsMode(0)
	EventsModeOn  = EventsMode(1)
)

type Event struct {
	Key  string
	Data []byte
}
