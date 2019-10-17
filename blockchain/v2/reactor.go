package v2

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/behaviour"
	"github.com/tendermint/tendermint/libs/log"
)

type timeCheck struct {
	priorityHigh
	time time.Time
}

func schedulerHandle(event Event) (Event, error) {
	if _, ok := event.(timeCheck); ok {
		fmt.Println("scheduler handle timeCheck")
	}
	return noOp, nil
}

func processorHandle(event Event) (Event, error) {
	if _, ok := event.(timeCheck); ok {
		fmt.Println("processor handle timeCheck")
	}
	return noOp, nil
}

/*
	# What should we test

	# How should we test
		* Table based
		* Input Events
		* Assess reactor state
			* maxPeerHeight
				* from scheduler
			* syncHeight
				* from processor
			* BlockSynced
				* from processor
			* numActivePeers
				* from scheduler
	# How should we share state?
	* Option A: The FSMs (scheduler, processor, ...) export an API for secure access to their state
	* Option B:Routines output events which get consumed by a reporter
*/

type Reactor struct {
	events    chan Event // XXX: Rename eventsFromPeers
	stopDemux chan struct{}
	scheduler *Routine
	processor *Routine
	ticker    *time.Ticker
	logger    log.Logger
}

func NewReactor(bufferSize int) *Reactor {
	return &Reactor{
		events:    make(chan Event, bufferSize),
		stopDemux: make(chan struct{}),
		scheduler: newRoutine("scheduler", schedulerHandle, bufferSize),
		processor: newRoutine("processor", processorHandle, bufferSize),
		ticker:    time.NewTicker(1 * time.Second),
		logger:    log.NewNopLogger(),
	}
}

// nolint:unused
func (r *Reactor) setLogger(logger log.Logger) {
	r.logger = logger
	r.scheduler.setLogger(logger)
	r.processor.setLogger(logger)
}

func (r *Reactor) Start() {
	go r.scheduler.start()
	go r.processor.start()
	go r.demux()

	<-r.scheduler.ready()
	<-r.processor.ready()
}

func (r *Reactor) demux() {
	var (
		processBlockFreq = 1 * time.Second
		doProcessBlockCh = make(chan struct{}, 1)
		doProcessBlockTk = time.NewTicker(processFreq * time.Second)

		pruneFreq = 1 * time.Second
		doPruneCh = make(chan struct{}, 1)
		doPruneTk = time.NewTicker(pruneFreq * time.Second)

		scheduleFreq = 1 * time.Second
		doScheduleCh = make(chan struct{}, 1)
		doScheduleTk = time.NewTicker(scheduleFreq * time.Second)
	)

	for {
		select {
		// Pacers: send at most per freequency but don't saturate
		case _ <- doProcessBlockTk.C:
			select {
			case doProcessBlockCh <- struct{}{}:
			default:
			}
		case _ <- doPruneTk.C:
			select {
			case doPruneCh <- struct{}{}:
			default:
			}
		case _ <- doPruneTk.C:
			select {
			case doPruneCh <- struct{}{}:
			default:
			}

		// Tickers: perform tasks periodically
		case _ <- doScheduleTickerCh:
			r.scheduler.send(trySchedule{time: time.Now()})
		case _ <- doPrunePeerCh:
			r.scheduler.send(tryPrunePeer{time: time.Now()})
		case _ <- doProcessBlockCh:
			r.processor.send(pcProcessBlock{})

		// Events from peers
		case event := <-r.events:
			switch event := event.(type) {
			case bcStatusResponse, bcBlockRequestMessage:
				r.setMaxPeerHeight(event.height)
				// XXX: check for backpressure
				r.scheduler.send(event)
			}

		// Incremental events form scheduler
		case event := <-r.scheduler.next():
			switch event := event.(type) {
			case *scBlockReceived:
				r.processor.send(event)
			case scPeerError:
				r.processor.send(event)
				r.swReporter.Report(behaviour.BadMessage(event.peerID, event.reason))
			case scBlockRequest:
				sent := r.io.sendBlockRequest(event.peerID, event.height)
				if !sent {
					// backpressure
				}
			}

		// Incremental events from processor
		case event := <-r.processor.next():
			// io.handle reportPeerError
			switch event := event.(type) {
			case pcBlockProcessed:
				r.setSyncHeight(event.height)
			case pcBlockVerificationFailure:
				r.scheduler.send(event)
			}

		// Terminal events from scheduler
		case err := <-r.scheduler.final():
			r.logger.Info(fmt.Sprintf("scheduler final %s", err))
			// send the processor stop?

		// Terminal event from processor
		case err := <-r.processor.final():
			r.logger.Info(fmt.Sprintf("processor final %s", err))
			if event.(pcFinished) {
				r.io.switchToConsensus(event.state, event.blocksSynced)
				// Stop the demuxer here?
			}
		case <-r.stopDemux:
			r.logger.Info("demuxing stopped")
			return
		}
	}
}

func (r *Reactor) Stop() {
	r.logger.Info("reactor stopping")

	r.ticker.Stop()
	r.scheduler.stop()
	r.processor.stop()
	close(r.stopDemux)
	close(r.events)

	r.logger.Info("reactor stopped")
}

func (r *Reactor) Receive(event Event) {
	// XXX: decode and serialize write events
	// TODO: backpressure
	r.events <- event
}

func (r *Reactor) AddPeer() {
	// TODO: add peer event and send to demuxer
}
