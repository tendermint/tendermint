package v2

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tendermint/tendermint/behaviour"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type bcBlockRequestMessage struct {
	priorityNormal
	Height int64 // XXX: could this be private?
}

func (m *bcBlockRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

type blockStore interface {
	LoadBlock(height int64) *types.Block
	SaveBlock(*types.Block, *types.PartSet, *types.Commit)
}

type Reactor struct {
	events    chan Event // XXX: Rename eventsFromPeers
	stopDemux chan struct{}
	scheduler *Routine
	processor *Routine
	logger    log.Logger

	mtx           sync.RWMutex
	maxPeerHeight int64
	syncHeight    int64

	reporter behaviour.Reporter
	io       iIo
	store    blockStore
}

type blockApplier interface {
	ApplyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error)
}

// XXX: unify naming in this package around tdState
// XXX: V1 stores a copy of state as initialState, which is never mutated. Is that nessesary?
func NewReactor(state state.State, store blockStore, reporter behaviour.Reporter, blockApplier blockApplier, bufferSize int) *Reactor {
	pContext := newProcessorContext(store, blockApplier, state)
	scheduler := newScheduler(state.LastBlockHeight)
	processor := newPcState(state, pContext)
	return &Reactor{
		events:    make(chan Event, bufferSize),
		stopDemux: make(chan struct{}),
		scheduler: newRoutine("scheduler", scheduler.handle, bufferSize),
		processor: newRoutine("processor", processor.handle, bufferSize),
		store:     store,
		reporter:  reporter,
		logger:    log.NewNopLogger(),
	}
}

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

		statusFreq = 1 * time.Second
		doStatusCh = make(chan struct{}, 1)
		doStatusTk = time.NewTicker(statusFreq * time.Second)
	)

	// XXX: Extract timers to make testing atemporal
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
		case <-doStatusTk.C:
			select {
			case doStatusCh <- struct{}{}:
			default:
			}

		// Tickers: perform tasks periodically
		case <-doScheduleCh:
			r.scheduler.send(trySchedule{time: time.Now()})
		case <-doPrunePeerCh:
			r.scheduler.send(tryPrunePeer{time: time.Now()})
		case <-doProcessBlockCh:
			r.processor.send(pcProcessBlock{})
		case <-doStatusCh:
			r.io.broadcastStatusRequest(r.SyncHeight())

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
				r.reporter.Report(behaviour.BadMessage(event.peerID, "scPeerError"))
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
			msg, ok := event.(pcFinished)
			if ok {
				r.io.switchToConsensus(msg.tdState, msg.blocksSynced)
			}
		case <-r.stopDemux:
			r.logger.Info("demuxing stopped")
			return
		}
	}
}

func (r *Reactor) Stop() {
	r.logger.Info("reactor stopping")

	r.scheduler.stop()
	r.processor.stop()
	close(r.stopDemux)
	close(r.events)

	r.logger.Info("reactor stopped")
}

const (
	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
)

// BlockchainMessage is a generic message for this reactor.
type BlockchainMessage interface {
	ValidateBasic() error
}

func decodeMsg(bz []byte) (msg BlockchainMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

func (r *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		r.logger.Error("error decoding message",
			"src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		_ = r.reporter.Report(behaviour.BadMessage(src.ID(), err.Error()))
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		r.logger.Error("peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		_ = r.reporter.Report(behaviour.BadMessage(src.ID(), err.Error()))
		return
	}

	r.logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcBlockRequestMessage:
		block := r.store.LoadBlock(msg.Height)
		if block != nil {
			if err = r.io.sendBlockToPeer(block, src.ID()); err != nil {
				r.logger.Error("Could not send block message to peer: ", err)
			}
		} else {
			r.logger.Info("peer asking for a block we don't have", "src", src, "height", msg.Height)
			if err = r.io.sendBlockNotFound(msg.Height, src.ID()); err != nil {
				r.logger.Error("Couldn't send block not found: ", err)
			}
		}
	case *bcStatusRequestMessage:
		if err := r.io.sendStatusResponse(r.SyncHeight(), src.ID()); err != nil {
			r.logger.Error("Could not send status message to peer", "src", src)
		}
	case Event:
		// Forward to state machines
		r.events <- msg
	}
}

func (r *Reactor) AddPeer(peer p2p.Peer) {
	err := r.io.sendStatusResponse(r.SyncHeight(), peer.ID())
	if err != nil {
		r.logger.Error("Could not send status message to peer new", "src", peer.ID, "height", r.SyncHeight())
	}
	r.events <- addNewPeer{peerID: peer.ID()}
}

type bcRemovePeer struct {
	priorityHigh
	peerID p2p.ID
	reason interface{}
}

func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	event := bcRemovePeer{
		peerID: peer.ID(),
		reason: reason,
	}

	r.events <- event
}
