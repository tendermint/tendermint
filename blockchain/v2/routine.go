package v2

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Workiva/go-datastructures/queue"

	"github.com/tendermint/tendermint/libs/log"
)

type handleFunc = func(event Event) (Event, error)

const historySize = 25

// Routine is a structure that models a finite state machine as serialized
// stream of events processed by a handle function. This Routine structure
// handles the concurrency and messaging guarantees. Events are sent via
// `send` are handled by the `handle` function to produce an iterator
// `next()`. Calling `stop()` on a routine will conclude processing of all
// sent events and produce `final()` event representing the terminal state.
type Routine struct {
	name    string
	handle  handleFunc
	queue   *queue.PriorityQueue
	history []Event
	out     chan Event
	fin     chan error
	rdy     chan struct{}
	running *uint32
	logger  log.Logger
	metrics *Metrics
}

func newRoutine(name string, handleFunc handleFunc, bufferSize int) *Routine {
	return &Routine{
		name:    name,
		handle:  handleFunc,
		queue:   queue.NewPriorityQueue(bufferSize, true),
		history: make([]Event, 0, historySize),
		out:     make(chan Event, bufferSize),
		rdy:     make(chan struct{}, 1),
		fin:     make(chan error, 1),
		running: new(uint32),
		logger:  log.NewNopLogger(),
		metrics: NopMetrics(),
	}
}

func (rt *Routine) setLogger(logger log.Logger) {
	rt.logger = logger
}

func (rt *Routine) start() {
	rt.logger.Info("routine start", "msg", log.NewLazySprintf("%s: run", rt.name))
	running := atomic.CompareAndSwapUint32(rt.running, uint32(0), uint32(1))
	if !running {
		panic(fmt.Sprintf("%s is already running", rt.name))
	}
	close(rt.rdy)
	defer func() {
		if r := recover(); r != nil {
			var (
				b strings.Builder
				j int
			)
			for i := len(rt.history) - 1; i >= 0; i-- {
				fmt.Fprintf(&b, "%d: %+v\n", j, rt.history[i])
				j++
			}
			panic(fmt.Sprintf("%v\nlast events:\n%v", r, b.String()))
		}
		stopped := atomic.CompareAndSwapUint32(rt.running, uint32(1), uint32(0))
		if !stopped {
			panic(fmt.Sprintf("%s is failed to stop", rt.name))
		}
	}()

	for {
		events, err := rt.queue.Get(1)
		if err == queue.ErrDisposed {
			rt.terminate(nil)
			return
		} else if err != nil {
			rt.terminate(err)
			return
		}
		oEvent, err := rt.handle(events[0].(Event))
		rt.metrics.EventsHandled.With("routine", rt.name).Add(1)
		if err != nil {
			rt.terminate(err)
			return
		}
		rt.metrics.EventsOut.With("routine", rt.name).Add(1)
		rt.logger.Debug("routine start", "msg", log.NewLazySprintf("%s: produced %T %+v", rt.name, oEvent, oEvent))

		// Skip rTrySchedule and rProcessBlock events as they clutter the history
		// due to their frequency.
		switch events[0].(type) {
		case rTrySchedule:
		case rProcessBlock:
		default:
			rt.history = append(rt.history, events[0].(Event))
			if len(rt.history) > historySize {
				rt.history = rt.history[1:]
			}
		}

		rt.out <- oEvent
	}
}

// XXX: look into returning OpError in the net package
func (rt *Routine) send(event Event) bool {
	rt.logger.Debug("routine send", "msg", log.NewLazySprintf("%s: received %T %+v", rt.name, event, event))
	if !rt.isRunning() {
		return false
	}
	err := rt.queue.Put(event)
	if err != nil {
		rt.metrics.EventsShed.With("routine", rt.name).Add(1)
		rt.logger.Error(fmt.Sprintf("%s: send failed, queue was full/stopped", rt.name))
		return false
	}

	rt.metrics.EventsSent.With("routine", rt.name).Add(1)
	return true
}

func (rt *Routine) isRunning() bool {
	return atomic.LoadUint32(rt.running) == 1
}

func (rt *Routine) next() chan Event {
	return rt.out
}

func (rt *Routine) ready() chan struct{} {
	return rt.rdy
}

func (rt *Routine) stop() {
	if !rt.isRunning() { // XXX: this should check rt.queue.Disposed()
		return
	}

	rt.logger.Info("routine stop", "msg", log.NewLazySprintf("%s: stop", rt.name))
	rt.queue.Dispose() // this should block until all queue items are free?
}

func (rt *Routine) final() chan error {
	return rt.fin
}

// XXX: Maybe get rid of this
func (rt *Routine) terminate(reason error) {
	// We don't close the rt.out channel here, to avoid spinning on the closed channel
	// in the event loop.
	rt.fin <- reason
}
