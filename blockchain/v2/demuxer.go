// nolint:unused
package v2

import (
	"fmt"
	"sync/atomic"

	"github.com/tendermint/tendermint/libs/log"
)

type scFull struct{}
type pcFull struct{}

const demuxerBufferSize = 10

type demuxer struct {
	input     chan Event
	scheduler *Routine
	processor *Routine
	fin       chan error
	stopped   chan struct{}
	rdy       chan struct{}
	running   *uint32
	stopping  *uint32
	logger    log.Logger
}

func newDemuxer(scheduler *Routine, processor *Routine) *demuxer {
	return &demuxer{
		input:     make(chan Event, demuxerBufferSize),
		scheduler: scheduler,
		processor: processor,
		stopped:   make(chan struct{}, 1),
		fin:       make(chan error, 1),
		rdy:       make(chan struct{}, 1),
		running:   new(uint32),
		stopping:  new(uint32),
		logger:    log.NewNopLogger(),
	}
}

func (dm *demuxer) setLogger(logger log.Logger) {
	dm.logger = logger
}

func (dm *demuxer) start() {
	starting := atomic.CompareAndSwapUint32(dm.running, uint32(0), uint32(1))
	if !starting {
		panic("Routine has already started")
	}
	dm.logger.Info("demuxer: run")
	dm.rdy <- struct{}{}
	for {
		if !dm.isRunning() {
			break
		}
		select {
		case event := <-dm.input:
			if _, ok := event.(stopEv); ok {
				dm.logger.Info("demuxer: stopping")
				dm.terminate(fmt.Errorf("stopped"))
				dm.stopped <- struct{}{}
				return
			}
			oEvent, err := dm.handle(event)
			if err != nil {
				dm.terminate(err)
				return
			}
			if !dm.isStopping() {
				dm.input <- oEvent // here
			}
		case event, ok := <-dm.scheduler.next():
			if !ok {
				dm.logger.Info("demuxer: scheduler output closed")
				continue
			}
			oEvent, err := dm.handle(event)
			if err != nil {
				dm.terminate(err)
				return
			}

			if !dm.isStopping() {
				dm.input <- oEvent
			}
		case event, ok := <-dm.processor.next():
			if !ok {
				dm.logger.Info("demuxer: processor output closed")
				continue
			}
			oEvent, err := dm.handle(event)
			if err != nil {
				dm.terminate(err)
				return
			}
			dm.input <- oEvent
		}
	}
}

func (dm *demuxer) handle(event Event) (Event, error) {
	received := dm.scheduler.trySend(event)
	if !received {
		return scFull{}, nil // backpressure
	}

	received = dm.processor.trySend(event)
	if !received {
		return pcFull{}, nil // backpressure
	}

	return NoOp{}, nil
}

func (dm *demuxer) trySend(event Event) bool {
	if !dm.isRunning() || dm.isStopping() {
		dm.logger.Info("dummuxer isn't running")
		return false
	}
	select {
	case dm.input <- event:
		return true
	default:
		dm.logger.Info("demuxer channel was full")
		return false
	}
}

func (dm *demuxer) isRunning() bool {
	return atomic.LoadUint32(dm.running) == 1
}

func (dm *demuxer) isStopping() bool {
	return atomic.LoadUint32(dm.stopping) == 1
}

func (dm *demuxer) ready() chan struct{} {
	return dm.rdy
}

func (dm *demuxer) stop() {
	if !dm.isRunning() {
		return
	}
	stopping := atomic.CompareAndSwapUint32(dm.stopping, uint32(0), uint32(1))
	if !stopping {
		panic("Demuxer has already stopped")
	}
	dm.logger.Info("demuxer stop")
	dm.input <- stopEv{}
	<-dm.stopped
}

func (dm *demuxer) terminate(reason error) {
	stopped := atomic.CompareAndSwapUint32(dm.running, uint32(1), uint32(0))
	if !stopped {
		panic("called terminate but already terminated")
	}
	dm.fin <- reason
}

func (dm *demuxer) final() chan error {
	return dm.fin
}
