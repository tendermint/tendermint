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

type Reactor struct {
	demuxer   *demuxer
	scheduler *Routine
	processor *Routine
	ticker    *time.Ticker
}

// nolint:unused
func (r *Reactor) setLogger(logger log.Logger) {
	r.scheduler.setLogger(logger)
	r.processor.setLogger(logger)
	r.demuxer.setLogger(logger)
}

func (r *Reactor) Start() {
	r.scheduler = newRoutine("scheduler", schedulerHandle)
	r.processor = newRoutine("processor", processorHandle)
	r.demuxer = newDemuxer(r.scheduler, r.processor)
	r.ticker = time.NewTicker(1 * time.Second)

	go r.scheduler.start()
	go r.processor.start()
	go r.demuxer.start()

	<-r.scheduler.ready()
	<-r.processor.ready()
	<-r.demuxer.ready()

	go func() {
		for t := range r.ticker.C {
			r.demuxer.trySend(timeCheck{t})
		}
	}()
}

func (r *Reactor) Wait() {
	fmt.Println("completed routines")
	r.Stop()
}

func (r *Reactor) Stop() {
	fmt.Println("reactor stopping")

	r.ticker.Stop()
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
