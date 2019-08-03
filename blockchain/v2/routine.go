package v2

import (
	"fmt"
	"sync/atomic"

	"github.com/tendermint/tendermint/libs/log"
)

// TODO
// * revisit panic conditions
// * audit log levels
// * maybe routine should be an interface and the concret tpe should be handlerRoutine

// Adding Metrics
// we need a metrics definition
type handleFunc = func(event Event) (Events, error)

type Routine struct {
	name     string
	input    chan Event
	errors   chan error
	output   chan Event
	stopped  chan struct{}
	finished chan error
	running  *uint32
	handle   handleFunc
	logger   log.Logger
	metrics  *Metrics
}

func newRoutine(name string, output chan Event, handleFunc handleFunc) *Routine {
	return &Routine{
		name:     name,
		input:    make(chan Event, 1),
		handle:   handleFunc,
		errors:   make(chan error, 1),
		output:   output,
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
				rt.output <- event
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
				rt.output <- event
			}
		}
	}
}
func (rt *Routine) feedback() {
	for event := range rt.output {
		rt.send(event)
	}
}

func (rt *Routine) send(event Event) bool {
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

// rename flush?
func (rt *Routine) stop() {
	// XXX: what if already stopped?
	rt.logger.Info(fmt.Sprintf("%s: stop\n", rt.name))
	close(rt.input)
	close(rt.errors)
	<-rt.stopped
	rt.terminate(fmt.Errorf("routine stopped"))
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
