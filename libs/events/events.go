/*
Pub-Sub in go with event caching
*/
package events

import (
	"sync"

	cmn "github.com/tendermint/tendermint/libs/common"
)

// Generic event data can be typed and registered with tendermint/go-amino
// via concrete implementation of this interface
type EventData interface {
	//AssertIsEventData()
}

// reactors and other modules should export
// this interface to become eventable
type Eventable interface {
	SetEventSwitch(evsw EventSwitch)
}

// an event switch or cache implements fireable
type Fireable interface {
	FireEvent(event string, data EventData)
}

type EventSwitch interface {
	cmn.Service
	Fireable

	AddListenerForEvent(listenerID, event string, cb EventCallback)
	RemoveListenerForEvent(event string, listenerID string)
	RemoveListener(listenerID string)
}

type eventSwitch struct {
	cmn.BaseService

	mtx        sync.RWMutex
	eventCells map[string]*eventCell
	listeners  map[string]*eventListener
}

func NewEventSwitch() EventSwitch {
	evsw := &eventSwitch{
		eventCells: make(map[string]*eventCell),
		listeners:  make(map[string]*eventListener),
	}
	evsw.BaseService = *cmn.NewBaseService(nil, "EventSwitch", evsw)
	return evsw
}

func (evsw *eventSwitch) OnStart() error {
	return nil
}

func (evsw *eventSwitch) OnStop() {}

func (evsw *eventSwitch) AddListenerForEvent(listenerID, event string, cb EventCallback) {
	// Get/Create eventCell and listener
	evsw.mtx.Lock()
	eventCell := evsw.eventCells[event]
	if eventCell == nil {
		eventCell = newEventCell()
		evsw.eventCells[event] = eventCell
	}
	listener := evsw.listeners[listenerID]
	if listener == nil {
		listener = newEventListener(listenerID)
		evsw.listeners[listenerID] = listener
	}
	evsw.mtx.Unlock()

	// Add event and listener
	eventCell.AddListener(listenerID, cb)
	listener.AddEvent(event)
}

func (evsw *eventSwitch) RemoveListener(listenerID string) {
	// Get and remove listener
	evsw.mtx.RLock()
	listener := evsw.listeners[listenerID]
	evsw.mtx.RUnlock()
	if listener == nil {
		return
	}

	evsw.mtx.Lock()
	delete(evsw.listeners, listenerID)
	evsw.mtx.Unlock()

	// Remove callback for each event.
	listener.SetRemoved()
	for _, event := range listener.GetEvents() {
		evsw.RemoveListenerForEvent(event, listenerID)
	}
}

func (evsw *eventSwitch) RemoveListenerForEvent(event string, listenerID string) {
	// Get eventCell
	evsw.mtx.Lock()
	eventCell := evsw.eventCells[event]
	evsw.mtx.Unlock()

	if eventCell == nil {
		return
	}

	// Remove listenerID from eventCell
	numListeners := eventCell.RemoveListener(listenerID)

	// Maybe garbage collect eventCell.
	if numListeners == 0 {
		// Lock again and double check.
		evsw.mtx.Lock()      // OUTER LOCK
		eventCell.mtx.Lock() // INNER LOCK
		if len(eventCell.listeners) == 0 {
			delete(evsw.eventCells, event)
		}
		eventCell.mtx.Unlock() // INNER LOCK
		evsw.mtx.Unlock()      // OUTER LOCK
	}
}

func (evsw *eventSwitch) FireEvent(event string, data EventData) {
	// Get the eventCell
	evsw.mtx.RLock()
	eventCell := evsw.eventCells[event]
	evsw.mtx.RUnlock()

	if eventCell == nil {
		return
	}

	// Fire event for all listeners in eventCell
	eventCell.FireEvent(data)
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

func (cell *eventCell) FireEvent(data EventData) {
	cell.mtx.RLock()
	for _, listener := range cell.listeners {
		listener(data)
	}
	cell.mtx.RUnlock()
}

//-----------------------------------------------------------------------------

type EventCallback func(data EventData)

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

func (evl *eventListener) AddEvent(event string) {
	evl.mtx.Lock()
	defer evl.mtx.Unlock()

	if evl.removed {
		return
	}
	evl.events = append(evl.events, event)
}

func (evl *eventListener) GetEvents() []string {
	evl.mtx.RLock()
	defer evl.mtx.RUnlock()

	events := make([]string, len(evl.events))
	copy(events, evl.events)
	return events
}

func (evl *eventListener) SetRemoved() {
	evl.mtx.Lock()
	defer evl.mtx.Unlock()
	evl.removed = true
}
