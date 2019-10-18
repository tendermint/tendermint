package v2

import (
	"fmt"
	"sync"
	"time"

	"github.com/tendermint/tendermint/behaviour"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/state"
)

type bcBlockRequestMessage struct {
	priorityNormal
	Height int64 // XXX: could this be private?
}

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

type Reactor struct {
	events    chan Event // XXX: Rename eventsFromPeers
	stopDemux chan struct{}
	scheduler *Routine
	processor *Routine
	ticker    *time.Ticker
	logger    log.Logger

	mtx           sync.RWMutex
	maxPeerHeight int64
	syncHeight    int64

	swReporter *behaviour.SwitchReporter
	io         iIo
	state      state.State
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

// State
func (r *Reactor) setMaxPeerHeight(height int64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if height > r.maxPeerHeight {
		r.maxPeerHeight = height
	}
}

func (r *Reactor) MaxPeerHeight() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.maxPeerHeight
}

func (r *Reactor) setSyncHeight(height int64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.syncHeight = height
}

func (r *Reactor) SyncHeight() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.syncHeight
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
		doProcessBlockTk = time.NewTicker(processBlockFreq * time.Second)

		prunePeerFreq = 1 * time.Second
		doPrunePeerCh = make(chan struct{}, 1)
		doPrunePeerTk = time.NewTicker(prunePeerFreq * time.Second)

		scheduleFreq = 1 * time.Second
		doScheduleCh = make(chan struct{}, 1)
		doScheduleTk = time.NewTicker(scheduleFreq * time.Second)
		// XXX: add broadcastStatusRequest
	)

	for {
		select {
		// Pacers: send at most per freequency but don't saturate
		case <-doProcessBlockTk.C:
			select {
			case doProcessBlockCh <- struct{}{}:
			default:
			}
		case <-doPrunePeerTk.C:
			select {
			case doPrunePeerCh <- struct{}{}:
			default:
			}
		case <-doScheduleTk.C:
			select {
			case doScheduleCh <- struct{}{}:
			default:
			}

		// Tickers: perform tasks periodically
		case <-doScheduleCh:
			r.scheduler.send(trySchedule{time: time.Now()})
		case <-doPrunePeerCh:
			r.scheduler.send(tryPrunePeer{time: time.Now()})
		case <-doProcessBlockCh:
			r.processor.send(pcProcessBlock{})

		// Events from peers
		case event := <-r.events:
			switch event := event.(type) {
			case bcStatusResponse:
				r.setMaxPeerHeight(event.height)
				r.scheduler.send(event)
			case bcBlockRequestMessage:
				r.scheduler.send(event)
			}

		// Incremental events form scheduler
		case event := <-r.scheduler.next():
			switch event := event.(type) {
			case *scBlockReceived:
				r.processor.send(event)
			case scPeerError:
				r.processor.send(event)
				r.swReporter.Report(behaviour.BadMessage(event.peerID, "scPeerError"))
			case scBlockRequest:
				r.io.sendBlockRequest(event.peerID, event.height)
			}

		// Incremental events from processor
		case event := <-r.processor.next():
			// io.handle reportPeerError
			switch event := event.(type) {
			case pcBlockProcessed:
				r.setSyncHeight(event.height)
				r.scheduler.send(event)
			case pcBlockVerificationFailure:
				r.scheduler.send(event)
			}

		// Terminal events from scheduler
		case err := <-r.scheduler.final():
			r.logger.Info(fmt.Sprintf("scheduler final %s", err))
			// send the processor stop?

		// Terminal event from processor
		case event := <-r.processor.final():
			r.logger.Info(fmt.Sprintf("processor final %s", event))
			event, ok := event.(pcFinished)
			if ok {
				// TODO
				//r.io.switchToConsensus(r.state, event.blocksSynced)
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
	// what about send block?
	r.events <- event
}

func (r *Reactor) AddPeer() {
	// TODO: add peer event and send to demuxer
}
