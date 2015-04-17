package events

import (
	"sync"
	"sync/atomic"
)

// reactors and other modules should export
// this interface to become eventable
type Eventable interface {
	SetFireable(Fireable)
}

// an event switch or cache implements fireable
type Fireable interface {
	FireEvent(event string, msg interface{})
}

type EventSwitch struct {
	mtx        sync.RWMutex
	eventCells map[string]*eventCell
	listeners  map[string]*eventListener
	running    uint32
	quit       chan struct{}
}

func (evsw *EventSwitch) Start() {
	if atomic.CompareAndSwapUint32(&evsw.running, 0, 1) {
		evsw.eventCells = make(map[string]*eventCell)
		evsw.listeners = make(map[string]*eventListener)
		evsw.quit = make(chan struct{})
	}
}

func (evsw *EventSwitch) Stop() {
	if atomic.CompareAndSwapUint32(&evsw.running, 1, 0) {
		evsw.eventCells = nil
		evsw.listeners = nil
		close(evsw.quit)
	}
}

func (evsw *EventSwitch) AddListenerForEvent(listenerId, event string, cb eventCallback) {
	// Get/Create eventCell and listener
	evsw.mtx.Lock()
	eventCell := evsw.eventCells[event]
	if eventCell == nil {
		eventCell = newEventCell()
		evsw.eventCells[event] = eventCell
	}
	listener := evsw.listeners[listenerId]
	if listener == nil {
		listener = newEventListener(listenerId)
		evsw.listeners[listenerId] = listener
	}
	evsw.mtx.Unlock()

	// Add event and listener
	eventCell.AddListener(listenerId, cb)
	listener.AddEvent(event)
}

func (evsw *EventSwitch) RemoveListener(listenerId string) {
	// Get and remove listener
	evsw.mtx.RLock()
	listener := evsw.listeners[listenerId]
	delete(evsw.listeners, listenerId)
	evsw.mtx.RUnlock()

	if listener == nil {
		return
	}

	// Remove callback for each event.
	listener.SetRemoved()
	for _, event := range listener.GetEvents() {
		evsw.RemoveListenerForEvent(event, listenerId)
	}
}

func (evsw *EventSwitch) RemoveListenerForEvent(event string, listenerId string) {
	// Get eventCell
	evsw.mtx.Lock()
	eventCell := evsw.eventCells[event]
	evsw.mtx.Unlock()

	if eventCell == nil {
		return
	}

	// Remove listenerId from eventCell
	numListeners := eventCell.RemoveListener(listenerId)

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

func (evsw *EventSwitch) FireEvent(event string, msg interface{}) {
	// Get the eventCell
	evsw.mtx.RLock()
	eventCell := evsw.eventCells[event]
	evsw.mtx.RUnlock()

	if eventCell == nil {
		return
	}

	// Fire event for all listeners in eventCell
	eventCell.FireEvent(msg)
}

//-----------------------------------------------------------------------------

// eventCell handles keeping track of listener callbacks for a given event.
type eventCell struct {
	mtx       sync.RWMutex
	listeners map[string]eventCallback
}

func newEventCell() *eventCell {
	return &eventCell{
		listeners: make(map[string]eventCallback),
	}
}

func (cell *eventCell) AddListener(listenerId string, cb eventCallback) {
	cell.mtx.Lock()
	cell.listeners[listenerId] = cb
	cell.mtx.Unlock()
}

func (cell *eventCell) RemoveListener(listenerId string) int {
	cell.mtx.Lock()
	delete(cell.listeners, listenerId)
	numListeners := len(cell.listeners)
	cell.mtx.Unlock()
	return numListeners
}

func (cell *eventCell) FireEvent(msg interface{}) {
	cell.mtx.RLock()
	for _, listener := range cell.listeners {
		listener(msg)
	}
	cell.mtx.RUnlock()
}

//-----------------------------------------------------------------------------

type eventCallback func(msg interface{})

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
