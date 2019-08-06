package v2

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
)

func schedulerHandle(event Event) (Events, error) {
	switch event.(type) {
	case timeCheck:
		fmt.Println("scheduler handle timeCheck")
	case testEvent:
		fmt.Println("scheduler handle testEvent")
		return Events{scTestEvent{}}, nil
	}
	return Events{}, nil
}

func processorHandle(event Event) (Events, error) {
	switch event.(type) {
	case timeCheck:
		fmt.Println("processor handle timeCheck")
	case testEvent:
		fmt.Println("processor handle testEvent")
	case scTestEvent:
		fmt.Println("processor handle scTestEvent")
		return Events{}, fmt.Errorf("processor done")
	}
	return Events{}, nil
}

// reactor
type Reactor struct {
	demuxer       *demuxer
	scheduler     *Routine
	processor     *Routine
	ticker        *time.Ticker
	tickerStopped chan struct{}
}

func (r *Reactor) Start() {
	logger := log.TestingLogger()

	// what is the best way to get the events out of the routine
	r.scheduler = newRoutine("scheduler", schedulerHandle)
	r.scheduler.setLogger(logger)
	r.processor = newRoutine("processor", processorHandle)
	r.processor.setLogger(logger)
	// so actually the demuxer only needs to read from events
	r.demuxer = newDemuxer(r.scheduler, r.processor)
	r.tickerStopped = make(chan struct{})

	go r.scheduler.run()
	go r.processor.run()
	go r.demuxer.run()

	for {
		if r.scheduler.isRunning() && r.processor.isRunning() && r.demuxer.isRunning() {
			fmt.Println("routines running")
			break
		}
		fmt.Println("waiting")
		time.Sleep(1 * time.Second)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				r.demuxer.send(timeCheck{})
			case <-r.tickerStopped:
				fmt.Println("ticker stopped")
				return
			}
		}
	}()
}

func (r *Reactor) Wait() {
	fmt.Println("completed routines")
	r.Stop()
}

func (r *Reactor) Stop() {
	fmt.Println("reactor stopping")

	r.tickerStopped <- struct{}{}
	r.demuxer.stop()
	r.scheduler.stop()
	r.processor.stop()
	// todo: accumulator
	// todo: io

	fmt.Println("reactor stopped")
}

func (r *Reactor) Receive(event Event) {
	fmt.Println("receive event")
	sent := r.demuxer.send(event)
	if !sent {
		fmt.Println("demuxer is full")
	}
}

func (r *Reactor) AddPeer() {
	// TODO: add peer event and send to demuxer
}
