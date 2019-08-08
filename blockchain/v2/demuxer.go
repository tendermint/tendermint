package v2

import (
	"fmt"
	"sync/atomic"
)

type demuxer struct {
	input     chan Event
	scheduler *Routine
	processor *Routine
	fin       chan error
	stopped   chan struct{}
	running   *uint32
}

// TODO
// demuxer_test
// Termination process
// Logger
// Metrics
// Adhere to interface
func newDemuxer(scheduler *Routine, processor *Routine) *demuxer {
	return &demuxer{
		input:     make(chan Event, 10),
		scheduler: scheduler,
		processor: processor,
		stopped:   make(chan struct{}, 1),
		fin:       make(chan error, 1),
		running:   new(uint32),
	}
}

func (dm *demuxer) start() {
	starting := atomic.CompareAndSwapUint32(dm.running, uint32(0), uint32(1))
	if !starting {
		panic("Routine has already started")
	}
	fmt.Printf("demuxer: run\n")
	for {
		if !dm.isRunning() {
			break
		}
		select {
		case event, ok := <-dm.input:
			if !ok {
				fmt.Printf("demuxer: stopping\n")
				dm.terminate(fmt.Errorf("stopped"))
				dm.stopped <- struct{}{}
				return
			}
			oEvents, err := dm.handle(event)
			if err != nil {
				dm.terminate(err)
				return
			}
			for _, event := range oEvents {
				dm.input <- event
			}
		case event, ok := <-dm.scheduler.next():
			if !ok {
				fmt.Printf("demuxer: scheduler output closed\n")
				continue
			}
			oEvents, err := dm.handle(event)
			if err != nil {
				dm.terminate(err)
				return
			}
			for _, event := range oEvents {
				dm.input <- event
			}
		case event, ok := <-dm.processor.next():
			if !ok {
				fmt.Printf("demuxer: processor output closed\n")
				continue
			}
			oEvents, err := dm.handle(event)
			if err != nil {
				dm.terminate(err)
				return
			}
			for _, event := range oEvents {
				dm.input <- event
			}
		}
	}
}

func (dm *demuxer) handle(event Event) (Events, error) {
	received := dm.scheduler.trySend(event)
	if !received {
		return Events{scFull{}}, nil // backpressure
	}

	received = dm.processor.trySend(event)
	if !received {
		return Events{pcFull{}}, nil // backpressure
	}

	return Events{}, nil
}

func (dm *demuxer) trySend(event Event) bool {
	if !dm.isRunning() {
		fmt.Println("dummuxer isn't running")
		return false
	}
	select {
	case dm.input <- event:
		return true
	default:
		fmt.Printf("demuxer channel was full\n")
		return false
	}
}

func (dm *demuxer) isRunning() bool {
	return atomic.LoadUint32(dm.running) == 1
}

func (dm *demuxer) stop() {
	if !dm.isRunning() {
		return
	}
	fmt.Printf("demuxer stop\n")
	close(dm.input)
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
