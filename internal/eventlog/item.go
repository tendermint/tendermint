package eventlog

import (
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	"github.com/tendermint/tendermint/types"
)

// Cached constants for the pieces of reserved event names.
var (
	tmTypeTag string
	tmTypeKey string
)

func init() {
	parts := strings.SplitN(types.EventTypeKey, ".", 2)
	if len(parts) != 2 {
		panic("invalid event type key: " + types.EventTypeKey)
	}
	tmTypeTag = parts[0]
	tmTypeKey = parts[1]
}

// ABCIEventer is an optional extension interface that may be implemented by
// event data types, to expose ABCI metadata to the event log. If an event item
// does not implement this interface, it is presumed to have no ABCI metadata.
type ABCIEventer interface {
	// Return any ABCI events metadata the receiver contains.
	// The reported slice must not contain a type (tm.event) record, since some
	// events share the same structure among different event types.
	ABCIEvents() []abci.Event
}

// An Item is a single event item.
type Item struct {
	Cursor cursor.Cursor
	Type   string
	Data   types.EventData
	Events []abci.Event
}

// newItem constructs a new item with the specified cursor, type, and data.
func newItem(cursor cursor.Cursor, etype string, data types.EventData) *Item {
	return &Item{Cursor: cursor, Type: etype, Data: data, Events: makeEvents(etype, data)}
}

// makeEvents returns a slice of ABCI events comprising the type tag along with
// any internal events exported by the data value.
func makeEvents(etype string, data types.EventData) []abci.Event {
	base := []abci.Event{{
		Type: tmTypeTag,
		Attributes: []abci.EventAttribute{{
			Key: tmTypeKey, Value: etype,
		}},
	}}
	if evt, ok := data.(ABCIEventer); ok {
		return append(base, evt.ABCIEvents()...)
	}
	return base
}

// FindType reports whether events contains a tm.event event, and if so returns
// its value, which is the type of the underlying event item.
func FindType(events []abci.Event) (string, bool) {
	for _, evt := range events {
		if evt.Type != tmTypeTag {
			continue
		}
		for _, attr := range evt.Attributes {
			if attr.Key == tmTypeKey {
				return attr.Value, true
			}
		}
	}
	return "", false
}
