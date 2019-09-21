package v2

import (
	"fmt"
	"sync/atomic"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/tendermint/tendermint/libs/log"
)

type handleFunc = func(event Event) (Event, error)

// Routines are a structure which model a finite state machine as serialized
// stream of events processed by a handle function. This Routine structure
// handles the concurrency and messaging guarantees. Events are sent via
// `send` are handled by the `handle` function to produce an iterator
// `next()`. Calling `close()` on a routine will conclude processing of all
// sent events and produce `final()` event representing the terminal state.
type Routine struct {
	name    string
	handle  handleFunc
	queue   *queue.PriorityQueue
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
		out:     make(chan Event, bufferSize),
		rdy:     make(chan struct{}, 1),
		fin:     make(chan error, 1),
		running: new(uint32),
		logger:  log.NewNopLogger(),
		metrics: NopMetrics(),
	}
}

// nolint: unused
func (rt *Routine) setLogger(logger log.Logger) {
	rt.logger = logger
}

// nolint:unused
func (rt *Routine) setMetrics(metrics *Metrics) {
	rt.metrics = metrics
}

func (rt *Routine) start() {
	rt.logger.Info(fmt.Sprintf("%s: run\n", rt.name))
	running := atomic.CompareAndSwapUint32(rt.running, uint32(0), uint32(1))
	if !running {
		panic(fmt.Sprintf("%s is already running", rt.name))
	}
	close(rt.rdy)
	defer func() {
		stopped := atomic.CompareAndSwapUint32(rt.running, uint32(1), uint32(0))
		if !stopped {
			panic(fmt.Sprintf("%s is failed to stop", rt.name))
		}
	}()

	for {
		events, err := rt.queue.Get(1)
		if err != nil {
			rt.logger.Info(fmt.Sprintf("%s: stopping\n", rt.name))
			rt.terminate(fmt.Errorf("stopped"))
			return
		}
		oEvent, err := rt.handle(events[0].(Event))
		rt.metrics.EventsHandled.With("routine", rt.name).Add(1)
		if err != nil {
			rt.terminate(err)
			return
		}
		rt.metrics.EventsOut.With("routine", rt.name).Add(1)
		rt.logger.Debug(fmt.Sprintf("%s produced %T %+v\n", rt.name, oEvent, oEvent))

		rt.out <- oEvent
	}
}

// XXX: look into returning OpError in the net package
func (rt *Routine) send(event Event) bool {
	rt.logger.Debug(fmt.Sprintf("%s: received %T %+v", rt.name, event, event))
	if !rt.isRunning() {
		return false
	}
	err := rt.queue.Put(event)
	if err != nil {
		rt.metrics.EventsShed.With("routine", rt.name).Add(1)
		rt.logger.Info(fmt.Sprintf("%s: send failed, queue was full/stopped \n", rt.name))
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
	if !rt.isRunning() {
		return
	}

	rt.logger.Info(fmt.Sprintf("%s: stop\n", rt.name))
	rt.queue.Dispose() // this should block until all queue items are free?
}

func (rt *Routine) final() chan error {
	return rt.fin
}

// XXX: Maybe get rid of this
func (rt *Routine) terminate(reason error) {
	close(rt.out)
	rt.fin <- reason
}
