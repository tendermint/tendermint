// Package events - Pub-Sub in go with event caching
package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/tendermint/tendermint/libs/log"
)

// ErrListenerWasRemoved is returned by AddEvent if the listener was removed.
type ErrListenerWasRemoved struct {
	listenerID string
}

// Error implements the error interface.
func (e ErrListenerWasRemoved) Error() string {
	return fmt.Sprintf("listener #%s was removed", e.listenerID)
}

// EventData is a generic event data can be typed and registered with
// tendermint/go-amino via concrete implementation of this interface.
type EventData interface{}

// Eventable is the interface reactors and other modules must export to become
// eventable.
type Eventable interface {
	SetEventSwitch(evsw EventSwitch)
}

// Fireable is the interface that wraps the FireEvent method.
//
// FireEvent fires an event with the given name and data.
type Fireable interface {
	FireEvent(ctx context.Context, eventValue string, data EventData)
}

// EventSwitch is the interface for synchronous pubsub, where listeners
// subscribe to certain events and, when an event is fired (see Fireable),
// notified via a callback function.
//
// Listeners are added by calling AddListenerForEvent function.
// They can be removed by calling either RemoveListenerForEvent or
// RemoveListener (for all events).
type EventSwitch interface {
	Fireable
	AddListenerForEvent(listenerID, eventValue string, cb EventCallback) error
}

type eventSwitch struct {
	mtx        sync.RWMutex
	eventCells map[string]*eventCell
	listeners  map[string]*eventListener
}

func NewEventSwitch(logger log.Logger) EventSwitch {
	evsw := &eventSwitch{
		eventCells: make(map[string]*eventCell),
		listeners:  make(map[string]*eventListener),
	}
	return evsw
}

func (evsw *eventSwitch) AddListenerForEvent(listenerID, eventValue string, cb EventCallback) error {
	// Get/Create eventCell and listener.
	evsw.mtx.Lock()

	eventCell := evsw.eventCells[eventValue]
	if eventCell == nil {
		eventCell = newEventCell()
		evsw.eventCells[eventValue] = eventCell
	}

	listener := evsw.listeners[listenerID]
	if listener == nil {
		listener = newEventListener(listenerID)
		evsw.listeners[listenerID] = listener
	}

	evsw.mtx.Unlock()

	if err := listener.AddEvent(eventValue); err != nil {
		return err
	}

	eventCell.AddListener(listenerID, cb)
	return nil
}

func (evsw *eventSwitch) FireEvent(ctx context.Context, event string, data EventData) {
	// Get the eventCell
	evsw.mtx.RLock()
	eventCell := evsw.eventCells[event]
	evsw.mtx.RUnlock()

	if eventCell == nil {
		return
	}

	// Fire event for all listeners in eventCell
	eventCell.FireEvent(ctx, data)
}

//-----------------------------------------------------------------------------

// eventCell handles keeping track of listener callbacks for a given event.
type eventCell struct {
	mtx       sync.RWMutex
	listeners map[string]EventCallback
}

func newEventCell() *eventCell {
	return &eventCell{
		listeners: make(map[string]EventCallback),
	}
}

func (cell *eventCell) AddListener(listenerID string, cb EventCallback) {
	cell.mtx.Lock()
	cell.listeners[listenerID] = cb
	cell.mtx.Unlock()
}

func (cell *eventCell) RemoveListener(listenerID string) int {
	cell.mtx.Lock()
	delete(cell.listeners, listenerID)
	numListeners := len(cell.listeners)
	cell.mtx.Unlock()
	return numListeners
}

func (cell *eventCell) FireEvent(ctx context.Context, data EventData) {
	cell.mtx.RLock()
	eventCallbacks := make([]EventCallback, 0, len(cell.listeners))
	for _, cb := range cell.listeners {
		eventCallbacks = append(eventCallbacks, cb)
	}
	cell.mtx.RUnlock()

	for _, cb := range eventCallbacks {
		if err := cb(ctx, data); err != nil {
			// should we log or abort here?
			continue
		}
	}
}

//-----------------------------------------------------------------------------

type EventCallback func(ctx context.Context, data EventData) error

type eventListener struct {
	id string

	mtx     sync.RWMutex
	removed bool
	events  []string
}

func newEventListener(id string) *eventListener {
	return &eventListener{
		id:      id,
		removed: false,
		events:  nil,
	}
}

func (evl *eventListener) AddEvent(event string) error {
	evl.mtx.Lock()

	if evl.removed {
		evl.mtx.Unlock()
		return ErrListenerWasRemoved{listenerID: evl.id}
	}

	evl.events = append(evl.events, event)
	evl.mtx.Unlock()
	return nil
}

func (evl *eventListener) GetEvents() []string {
	evl.mtx.RLock()
	events := make([]string, len(evl.events))
	copy(events, evl.events)
	evl.mtx.RUnlock()
	return events
}

func (evl *eventListener) SetRemoved() {
	evl.mtx.Lock()
	evl.removed = true
	evl.mtx.Unlock()
}
