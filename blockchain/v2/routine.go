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
// * determine the public interface

type handleFunc = func(event Event) (Events, error)

type Routine struct {
	name     string
	input    chan Event
	errors   chan error
	out      chan Event
	stopped  chan struct{}
	finished chan error
	running  *uint32
	handle   handleFunc
	logger   log.Logger
	metrics  *Metrics
}

func newRoutine(name string, handleFunc handleFunc) *Routine {
	return &Routine{
		name:     name,
		input:    make(chan Event, 1),
		handle:   handleFunc,
		errors:   make(chan error, 1),
		out:      make(chan Event, 1),
		stopped:  make(chan struct{}, 1),
		finished: make(chan error, 1),
		running:  new(uint32),
		logger:   log.NewNopLogger(),
		metrics:  NopMetrics(),
	}
}

func (rt *Routine) setLogger(logger log.Logger) {
	rt.logger = logger
}

func (rt *Routine) setMetrics(metrics *Metrics) {
	rt.metrics = metrics
}

func (rt *Routine) run() {
	rt.logger.Info(fmt.Sprintf("%s: run\n", rt.name))
	starting := atomic.CompareAndSwapUint32(rt.running, uint32(0), uint32(1))
	if !starting {
		panic("Routine has already started")
	}
	errorsDrained := false
	for {
		if !rt.isRunning() {
			break
		}
		select {
		case iEvent, ok := <-rt.input:
			rt.metrics.EventsIn.With("routine", rt.name).Add(1)
			if !ok {
				if !errorsDrained {
					continue // wait for errors to be drainned
				}
				rt.logger.Info(fmt.Sprintf("%s: stopping\n", rt.name))
				rt.stopped <- struct{}{}
				rt.terminate(fmt.Errorf("stopped"))
				return
			}
			oEvents, err := rt.handle(iEvent)
			rt.metrics.EventsHandled.With("routine", rt.name).Add(1)
			if err != nil {
				rt.terminate(err)
				return
			}
			rt.metrics.EventsOut.With("routine", rt.name).Add(float64(len(oEvents)))
			rt.logger.Info(fmt.Sprintf("%s handled %d events\n", rt.name, len(oEvents)))
			for _, event := range oEvents {
				rt.logger.Info(fmt.Sprintln("writting back to output"))
				rt.out <- event
			}
		case iEvent, ok := <-rt.errors:
			rt.metrics.ErrorsIn.With("routine", rt.name).Add(1)
			if !ok {
				rt.logger.Info(fmt.Sprintf("%s: errors closed\n", rt.name))
				errorsDrained = true
				continue
			}
			oEvents, err := rt.handle(iEvent)
			rt.metrics.ErrorsHandled.With("routine", rt.name).Add(1)
			if err != nil {
				rt.terminate(err)
				return
			}
			rt.metrics.ErrorsOut.With("routine", rt.name).Add(float64(len(oEvents)))
			for _, event := range oEvents {
				rt.out <- event
			}
		}
	}
}
func (rt *Routine) feedback() {
	for event := range rt.out {
		rt.send(event)
	}
}

// XXX: this should be called trySend for consistency
func (rt *Routine) send(event Event) bool {
	if !rt.isRunning() {
		return false
	}
	rt.logger.Info(fmt.Sprintf("%s: sending %+v", rt.name, event))
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
		case rt.input <- event:
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

func (rt *Routine) output() chan Event {
	return rt.out
}

func (rt *Routine) stop() {
	if !rt.isRunning() {
		return
	}
	rt.logger.Info(fmt.Sprintf("%s: stop\n", rt.name))
	close(rt.input)
	close(rt.errors)
	<-rt.stopped
}

func (rt *Routine) terminate(reason error) {
	stopped := atomic.CompareAndSwapUint32(rt.running, uint32(1), uint32(0))
	if !stopped {
		panic("called stop but already stopped")
	}
	rt.finished <- reason
}

// XXX: this should probably produced the finished
// channel and let the caller deicde how long to wait
func (rt *Routine) wait() error {
	return <-rt.finished
}
