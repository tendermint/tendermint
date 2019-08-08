package v2

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
)

type timeCheck struct {
	time time.Time
}

func schedulerHandle(event Event) (Events, error) {
	switch event.(type) {
	case timeCheck:
		fmt.Println("scheduler handle timeCheck")
	case Event:
		fmt.Println("scheduler handle testEvent")
	}
	return Events{}, nil
}

func processorHandle(event Event) (Events, error) {
	switch event.(type) {
	case timeCheck:
		fmt.Println("processor handle timeCheck")
	case Event:
		fmt.Println("processor handle event")
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

func (r *Reactor) setLogger(logger log.Logger) {
	r.scheduler.setLogger(logger)
	r.processor.setLogger(logger)
	r.demuxer.setLogger(logger)
}

func (r *Reactor) Start() {
	r.scheduler = newRoutine("scheduler", schedulerHandle)
	r.processor = newRoutine("processor", processorHandle)
	r.demuxer = newDemuxer(r.scheduler, r.processor)
	r.tickerStopped = make(chan struct{})

	go r.scheduler.start()
	go r.processor.start()
	go r.demuxer.start()

	for {
		if r.scheduler.isRunning() && r.processor.isRunning() && r.demuxer.isRunning() {
			fmt.Println("routines running")
			break
		}
		fmt.Println("waiting")
		time.Sleep(10 * time.Millisecond)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				r.demuxer.trySend(timeCheck{})
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
	sent := r.demuxer.trySend(event)
	if !sent {
		fmt.Println("demuxer is full")
	}
}

func (r *Reactor) AddPeer() {
	// TODO: add peer event and send to demuxer
}
