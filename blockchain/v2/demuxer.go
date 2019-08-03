package v2

import "fmt"

type demuxer struct {
	eventbus  chan Event
	scheduler *Routine
	processor *Routine
	finished  chan error
	stopped   chan struct{}
}

func newDemuxer(scheduler *Routine, processor *Routine) *demuxer {
	return &demuxer{
		eventbus:  make(chan Event, 10),
		scheduler: scheduler,
		processor: processor,
		stopped:   make(chan struct{}, 1),
		finished:  make(chan error, 1),
	}
}

// What should the termination clause be?
// Is any of the subroutines finishe, the demuxer finishes
func (dm *demuxer) run() {
	fmt.Printf("demuxer: run\n")
	for {
		select {
		case event, ok := <-dm.eventbus:
			if !ok {
				fmt.Printf("demuxer: stopping\n")
				dm.stopped <- struct{}{}
				return
			}
			oEvents, err := dm.handle(event)
			if err != nil {
				// TODO Termination time
				return
			}
			for _, event := range oEvents {
				dm.eventbus <- event
			}
		case event, ok := <-dm.scheduler.output:
			if !ok {
				fmt.Printf("demuxer: scheduler output closed\n")
				continue
			}
			oEvents, err := dm.handle(event)
			if err != nil {
				// TODO tTermination time
				return
			}
			for _, event := range oEvents {
				dm.eventbus <- event
			}
		case event, ok := <-dm.processor.output:
			if !ok {
				fmt.Printf("demuxer: processor output closed\n")
				continue
			}
			oEvents, err := dm.handle(event)
			if err != nil {
				// TODO tTermination time
				return
			}
			for _, event := range oEvents {
				dm.eventbus <- event
			}
		case err := <-dm.scheduler.finished:
			dm.finished <- err
		case err := <-dm.processor.finished:
			dm.finished <- err
		}
	}
}

func (dm *demuxer) handle(event Event) (Events, error) {
	received := dm.scheduler.send(event)
	if !received {
		return Events{scFull{}}, nil // backpressure
	}

	received = dm.processor.send(event)
	if !received {
		return Events{pcFull{}}, nil // backpressure
	}

	return Events{}, nil
}

func (dm *demuxer) send(event Event) bool {
	fmt.Printf("demuxer send\n")
	select {
	case dm.eventbus <- event:
		return true
	default:
		fmt.Printf("demuxer channel was full\n")
		return false
	}
}

func (dm *demuxer) stop() {
	fmt.Printf("demuxer stop\n")
	close(dm.eventbus)
	<-dm.stopped
	dm.terminate(fmt.Errorf("stopped"))
}

func (dm *demuxer) terminate(reason error) {
	dm.finished <- reason
}

func (dm *demuxer) wait() error {
	return <-dm.finished
}
