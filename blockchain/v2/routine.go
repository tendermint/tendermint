package v2

import (
	"fmt"
	"sync/atomic"

	"github.com/tendermint/tendermint/libs/log"
)

// TODO
// * revisit panic conditions
// * audit log levels
// * Convert routine to an interface with concrete implmentation

type handleFunc = func(event Event) (Event, error)

// Routines are a structure which model a finite state machine as serialized
// stream of events processed by a handle function. This Routine structure
// handles the concurrency and messaging guarantees. Events are sent via
// `trySend` are handled by the `handle` function to produce an iterator
// `next()`. Calling `close()` on a routine will conclude processing of all
// sent events and produce `last()` event representing the terminal state.
type Routine struct {
	name     string
	input    chan Event
	errors   chan error
	out      chan Event
	stopped  chan struct{}
	rdy      chan struct{}
	fin      chan error
	running  *uint32
	handle   handleFunc
	logger   log.Logger
	metrics  *Metrics
	stopping *uint32
}

func newRoutine(name string, handleFunc handleFunc) *Routine {
	return &Routine{
		name:     name,
		input:    make(chan Event, 10),
		handle:   handleFunc,
		errors:   make(chan error, 10),
		out:      make(chan Event, 10),
		stopped:  make(chan struct{}, 1),
		rdy:      make(chan struct{}, 1),
		fin:      make(chan error, 1),
		running:  new(uint32),
		stopping: new(uint32),
		logger:   log.NewNopLogger(),
		metrics:  NopMetrics(),
	}
}

func (rt *Routine) setLogger(logger log.Logger) {
	rt.logger = logger
}

// nolint:unused
func (rt *Routine) setMetrics(metrics *Metrics) {
	rt.metrics = metrics
}

func (rt *Routine) start() {
	rt.logger.Info(fmt.Sprintf("%s: run\n", rt.name))
	starting := atomic.CompareAndSwapUint32(rt.running, uint32(0), uint32(1))
	if !starting {
		panic("Routine has already started")
	}
	rt.rdy <- struct{}{}
	errorsDrained := false
	for {
		if !rt.isRunning() {
			rt.logger.Info(fmt.Sprintf("%s: breaking because not running\n", rt.name))
			break
		}
		select {
		case iEvent, ok := <-rt.input:
			rt.metrics.EventsIn.With("routine", rt.name).Add(1)
			if !ok {
				if !errorsDrained {
					rt.logger.Info(fmt.Sprintf("%s: waiting for errors to drain\n", rt.name))
					continue // wait for errors to be drainned
				}
				rt.logger.Info(fmt.Sprintf("%s: stopping\n", rt.name))
				close(rt.stopped)
				rt.terminate(fmt.Errorf("stopped"))
				return
			}
			oEvent, err := rt.handle(iEvent)
			rt.metrics.EventsHandled.With("routine", rt.name).Add(1)
			if err != nil {
				rt.terminate(err)
				return
			}
			rt.metrics.EventsOut.With("routine", rt.name).Add(1)
			rt.logger.Info(fmt.Sprintf("%s produced event: %#v\n", rt.name, oEvent))
			rt.out <- oEvent
		case iEvent, ok := <-rt.errors:
			rt.metrics.ErrorsIn.With("routine", rt.name).Add(1)
			if !ok {
				rt.logger.Info(fmt.Sprintf("%s: errors closed\n", rt.name))
				errorsDrained = true
				continue
			}
			oEvent, err := rt.handle(iEvent)
			rt.metrics.ErrorsHandled.With("routine", rt.name).Add(1)
			if err != nil {
				rt.terminate(err)
				return
			}
			rt.metrics.ErrorsOut.With("routine", rt.name).Add(1)
			rt.out <- oEvent
		}
	}
}
func (rt *Routine) feedback() {
	for event := range rt.out {
		rt.trySend(event)
	}
}

func (rt *Routine) trySend(event Event) bool {
	// XXX: need a mutex here
	// with a mutex, do we need is running?
	if !rt.isRunning() || rt.isStopping() {
		return false
	}

	rt.logger.Info(fmt.Sprintf("%s: sending %#v", rt.name, event))
	if err, ok := event.(error); ok {
		select {
		case rt.errors <- err:
			rt.metrics.ErrorsSent.With("routine", rt.name).Add(1)
			return true
		default:
			rt.metrics.ErrorsShed.With("routine", rt.name).Add(1)
			rt.logger.Info(fmt.Sprintf("%s: errors channel was full\n", rt.name))
			return false
		}
	} else {
		select {
		case rt.input <- event: // here
			rt.metrics.EventsSent.With("routine", rt.name).Add(1)
			return true
		default:
			rt.metrics.EventsShed.With("routine", rt.name).Add(1)
			rt.logger.Info(fmt.Sprintf("%s: channel was full\n", rt.name))
			return false
		}
	}
}

func (rt *Routine) isRunning() bool {
	return atomic.LoadUint32(rt.running) == 1
}

func (rt *Routine) isStopping() bool {
	return atomic.LoadUint32(rt.stopping) == 1
}

func (rt *Routine) ready() chan struct{} {
	return rt.rdy
}

func (rt *Routine) next() chan Event {
	return rt.out
}

// we need to ensure that no try send does not run while stop is running
func (rt *Routine) stop() {
	if !rt.isRunning() {
		return
	}

	rt.logger.Info(fmt.Sprintf("%s: stop\n", rt.name))
	stopping := atomic.CompareAndSwapUint32(rt.stopping, uint32(0), uint32(1))
	if !stopping {
		panic("Routine has already stopped")
	}

	close(rt.input) // here
	close(rt.errors)
	<-rt.stopped
}

func (rt *Routine) final() chan error {
	return rt.fin
}

func (rt *Routine) terminate(reason error) {
	stopped := atomic.CompareAndSwapUint32(rt.running, uint32(1), uint32(0))
	if !stopped {
		panic("called stop but already stopped")
	}
	rt.fin <- reason
}
