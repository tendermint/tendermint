package v2

import (
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/internal/blocksync"
	"github.com/tendermint/tendermint/internal/blocksync/v2/internal/behavior"
	"github.com/tendermint/tendermint/internal/consensus"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/sync"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
	"github.com/tendermint/tendermint/types"
)

const (
	// chBufferSize is the buffer size of all event channels.
	chBufferSize int = 1000
)

type blockStore interface {
	LoadBlock(height int64) *types.Block
	SaveBlock(*types.Block, *types.PartSet, *types.Commit)
	Base() int64
	Height() int64
}

// BlockchainReactor handles block sync protocol.
type BlockchainReactor struct {
	p2p.BaseReactor

	blockSync   *sync.AtomicBool // enable block sync on start when it's been Set
	stateSynced bool             // set to true when SwitchToBlockSync is called by state sync
	scheduler   *Routine
	processor   *Routine
	logger      log.Logger

	mtx           tmsync.RWMutex
	maxPeerHeight int64
	syncHeight    int64
	events        chan Event // non-nil during a block sync

	reporter behavior.Reporter
	io       iIO
	store    blockStore

	syncStartTime   time.Time
	syncStartHeight int64
	lastSyncRate    float64 // # blocks sync per sec base on the last 100 blocks
}

type blockApplier interface {
	ApplyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error)
}

// XXX: unify naming in this package around tmState
func newReactor(state state.State, store blockStore, reporter behavior.Reporter,
	blockApplier blockApplier, blockSync bool, metrics *consensus.Metrics) *BlockchainReactor {
	initHeight := state.LastBlockHeight + 1
	if initHeight == 1 {
		initHeight = state.InitialHeight
	}
	scheduler := newScheduler(initHeight, time.Now())
	pContext := newProcessorContext(store, blockApplier, state, metrics)
	// TODO: Fix naming to just newProcesssor
	// newPcState requires a processorContext
	processor := newPcState(pContext)

	return &BlockchainReactor{
		scheduler:       newRoutine("scheduler", scheduler.handle, chBufferSize),
		processor:       newRoutine("processor", processor.handle, chBufferSize),
		store:           store,
		reporter:        reporter,
		logger:          log.NewNopLogger(),
		blockSync:       sync.NewBool(blockSync),
		syncStartHeight: initHeight,
		syncStartTime:   time.Time{},
		lastSyncRate:    0,
	}
}

// NewBlockchainReactor creates a new reactor instance.
func NewBlockchainReactor(
	state state.State,
	blockApplier blockApplier,
	store blockStore,
	blockSync bool,
	metrics *consensus.Metrics) *BlockchainReactor {
	reporter := behavior.NewMockReporter()
	return newReactor(state, store, reporter, blockApplier, blockSync, metrics)
}

// SetSwitch implements Reactor interface.
func (r *BlockchainReactor) SetSwitch(sw *p2p.Switch) {
	r.Switch = sw
	if sw != nil {
		r.io = newSwitchIo(sw)
	} else {
		r.io = nil
	}
}

func (r *BlockchainReactor) setMaxPeerHeight(height int64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if height > r.maxPeerHeight {
		r.maxPeerHeight = height
	}
}

func (r *BlockchainReactor) setSyncHeight(height int64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.syncHeight = height
}

// SyncHeight returns the height to which the BlockchainReactor has synced.
func (r *BlockchainReactor) SyncHeight() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.syncHeight
}

// SetLogger sets the logger of the reactor.
func (r *BlockchainReactor) SetLogger(logger log.Logger) {
	r.logger = logger
	r.scheduler.setLogger(logger)
	r.processor.setLogger(logger)
}

// Start implements cmn.Service interface
func (r *BlockchainReactor) Start() error {
	r.reporter = behavior.NewSwitchReporter(r.BaseReactor.Switch)
	if r.blockSync.IsSet() {
		err := r.startSync(nil)
		if err != nil {
			return fmt.Errorf("failed to start block sync: %w", err)
		}
	}
	return nil
}

// startSync begins a block sync, signaled by r.events being non-nil. If state is non-nil,
// the scheduler and processor is updated with this state on startup.
func (r *BlockchainReactor) startSync(state *state.State) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.events != nil {
		return errors.New("block sync already in progress")
	}
	r.events = make(chan Event, chBufferSize)
	go r.scheduler.start()
	go r.processor.start()
	if state != nil {
		<-r.scheduler.ready()
		<-r.processor.ready()
		r.scheduler.send(bcResetState{state: *state})
		r.processor.send(bcResetState{state: *state})
	}
	go r.demux(r.events)
	return nil
}

// endSync ends a block sync
func (r *BlockchainReactor) endSync() {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.events != nil {
		close(r.events)
	}
	r.events = nil
	r.scheduler.stop()
	r.processor.stop()
}

// SwitchToBlockSync is called by the state sync reactor when switching to block sync.
func (r *BlockchainReactor) SwitchToBlockSync(state state.State) error {
	r.stateSynced = true
	state = state.Copy()

	err := r.startSync(&state)
	if err == nil {
		r.syncStartTime = time.Now()
	}

	return err
}

// reactor generated ticker events:
// ticker for cleaning peers
type rTryPrunePeer struct {
	priorityHigh
	time time.Time
}

func (e rTryPrunePeer) String() string {
	return fmt.Sprintf("rTryPrunePeer{%v}", e.time)
}

// ticker event for scheduling block requests
type rTrySchedule struct {
	priorityHigh
	time time.Time
}

func (e rTrySchedule) String() string {
	return fmt.Sprintf("rTrySchedule{%v}", e.time)
}

// ticker for block processing
type rProcessBlock struct {
	priorityNormal
}

func (e rProcessBlock) String() string {
	return "rProcessBlock"
}

// reactor generated events based on blockchain related messages from peers:
// blockResponse message received from a peer
type bcBlockResponse struct {
	priorityNormal
	time   time.Time
	peerID types.NodeID
	size   int64
	block  *types.Block
}

func (resp bcBlockResponse) String() string {
	return fmt.Sprintf("bcBlockResponse{%d#%X (size: %d bytes) from %v at %v}",
		resp.block.Height, resp.block.Hash(), resp.size, resp.peerID, resp.time)
}

// blockNoResponse message received from a peer
type bcNoBlockResponse struct {
	priorityNormal
	time   time.Time
	peerID types.NodeID
	height int64
}

func (resp bcNoBlockResponse) String() string {
	return fmt.Sprintf("bcNoBlockResponse{%v has no block at height %d at %v}",
		resp.peerID, resp.height, resp.time)
}

// statusResponse message received from a peer
type bcStatusResponse struct {
	priorityNormal
	time   time.Time
	peerID types.NodeID
	base   int64
	height int64
}

func (resp bcStatusResponse) String() string {
	return fmt.Sprintf("bcStatusResponse{%v is at height %d (base: %d) at %v}",
		resp.peerID, resp.height, resp.base, resp.time)
}

// new peer is connected
type bcAddNewPeer struct {
	priorityNormal
	peerID types.NodeID
}

func (resp bcAddNewPeer) String() string {
	return fmt.Sprintf("bcAddNewPeer{%v}", resp.peerID)
}

// existing peer is removed
type bcRemovePeer struct {
	priorityHigh
	peerID types.NodeID
	reason interface{}
}

func (resp bcRemovePeer) String() string {
	return fmt.Sprintf("bcRemovePeer{%v due to %v}", resp.peerID, resp.reason)
}

// resets the scheduler and processor state, e.g. following a switch from state syncing
type bcResetState struct {
	priorityHigh
	state state.State
}

func (e bcResetState) String() string {
	return fmt.Sprintf("bcResetState{%v}", e.state)
}

// Takes the channel as a parameter to avoid race conditions on r.events.
func (r *BlockchainReactor) demux(events <-chan Event) {
	var lastHundred = time.Now()

	var (
		processBlockFreq = 20 * time.Millisecond
		doProcessBlockCh = make(chan struct{}, 1)
		doProcessBlockTk = time.NewTicker(processBlockFreq)
	)
	defer doProcessBlockTk.Stop()

	var (
		prunePeerFreq = 1 * time.Second
		doPrunePeerCh = make(chan struct{}, 1)
		doPrunePeerTk = time.NewTicker(prunePeerFreq)
	)
	defer doPrunePeerTk.Stop()

	var (
		scheduleFreq = 20 * time.Millisecond
		doScheduleCh = make(chan struct{}, 1)
		doScheduleTk = time.NewTicker(scheduleFreq)
	)
	defer doScheduleTk.Stop()

	var (
		statusFreq = 10 * time.Second
		doStatusCh = make(chan struct{}, 1)
		doStatusTk = time.NewTicker(statusFreq)
	)
	defer doStatusTk.Stop()
	doStatusCh <- struct{}{} // immediately broadcast to get status of existing peers

	// Memoize the scSchedulerFail error to avoid printing it every scheduleFreq.
	var scSchedulerFailErr error

	// XXX: Extract timers to make testing atemporal
	for {
		select {
		// Pacers: send at most per frequency but don't saturate
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
			r.scheduler.send(rTrySchedule{time: time.Now()})
		case <-doPrunePeerCh:
			r.scheduler.send(rTryPrunePeer{time: time.Now()})
		case <-doProcessBlockCh:
			r.processor.send(rProcessBlock{})
		case <-doStatusCh:
			if err := r.io.broadcastStatusRequest(); err != nil {
				r.logger.Error("Error broadcasting status request", "err", err)
			}

		// Events from peers. Closing the channel signals event loop termination.
		case event, ok := <-events:
			if !ok {
				r.logger.Info("Stopping event processing")
				return
			}
			switch event := event.(type) {
			case bcStatusResponse:
				r.setMaxPeerHeight(event.height)
				r.scheduler.send(event)
			case bcAddNewPeer, bcRemovePeer, bcBlockResponse, bcNoBlockResponse:
				r.scheduler.send(event)
			default:
				r.logger.Error("Received unexpected event", "event", fmt.Sprintf("%T", event))
			}

		// Incremental events from scheduler
		case event := <-r.scheduler.next():
			switch event := event.(type) {
			case scBlockReceived:
				r.processor.send(event)
			case scPeerError:
				r.processor.send(event)
				if err := r.reporter.Report(behavior.BadMessage(event.peerID, "scPeerError")); err != nil {
					r.logger.Error("Error reporting peer", "err", err)
				}
			case scBlockRequest:
				peer := r.Switch.Peers().Get(event.peerID)
				if peer == nil {
					r.logger.Error("Wanted to send block request, but no such peer", "peerID", event.peerID)
					continue
				}
				if err := r.io.sendBlockRequest(peer, event.height); err != nil {
					r.logger.Error("Error sending block request", "err", err)
				}
			case scFinishedEv:
				r.processor.send(event)
				r.scheduler.stop()
			case scSchedulerFail:
				if scSchedulerFailErr != event.reason {
					r.logger.Error("Scheduler failure", "err", event.reason.Error())
					scSchedulerFailErr = event.reason
				}
			case scPeersPruned:
				// Remove peers from the processor.
				for _, peerID := range event.peers {
					r.processor.send(scPeerError{peerID: peerID, reason: errors.New("peer was pruned")})
				}
				r.logger.Debug("Pruned peers", "count", len(event.peers))
			case noOpEvent:
			default:
				r.logger.Error("Received unexpected scheduler event", "event", fmt.Sprintf("%T", event))
			}

		// Incremental events from processor
		case event := <-r.processor.next():
			switch event := event.(type) {
			case pcBlockProcessed:
				r.setSyncHeight(event.height)
				if (r.syncHeight-r.syncStartHeight)%100 == 0 {
					newSyncRate := 100 / time.Since(lastHundred).Seconds()
					if r.lastSyncRate == 0 {
						r.lastSyncRate = newSyncRate
					} else {
						r.lastSyncRate = 0.9*r.lastSyncRate + 0.1*newSyncRate
					}
					r.logger.Info("block sync Rate", "height", r.syncHeight,
						"max_peer_height", r.maxPeerHeight, "blocks/s", r.lastSyncRate)
					lastHundred = time.Now()
				}
				r.scheduler.send(event)
			case pcBlockVerificationFailure:
				r.scheduler.send(event)
			case pcFinished:
				r.logger.Info("block sync complete, switching to consensus")
				if !r.io.trySwitchToConsensus(event.tmState, event.blocksSynced > 0 || r.stateSynced) {
					r.logger.Error("Failed to switch to consensus reactor")
				}
				r.endSync()
				r.blockSync.UnSet()
				return
			case noOpEvent:
			default:
				r.logger.Error("Received unexpected processor event", "event", fmt.Sprintf("%T", event))
			}

		// Terminal event from scheduler
		case err := <-r.scheduler.final():
			switch err {
			case nil:
				r.logger.Info("Scheduler stopped")
			default:
				r.logger.Error("Scheduler aborted with error", "err", err)
			}

		// Terminal event from processor
		case err := <-r.processor.final():
			switch err {
			case nil:
				r.logger.Info("Processor stopped")
			default:
				r.logger.Error("Processor aborted with error", "err", err)
			}
		}
	}
}

// Stop implements cmn.Service interface.
func (r *BlockchainReactor) Stop() error {
	r.logger.Info("reactor stopping")
	r.endSync()
	r.logger.Info("reactor stopped")
	return nil
}

// Receive implements Reactor by handling different message types.
// XXX: do not call any methods that can block or incur heavy processing.
// https://github.com/tendermint/tendermint/issues/2888
func (r *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	logger := r.logger.With("src", src.ID(), "chID", chID)

	msgProto := new(bcproto.Message)

	if err := proto.Unmarshal(msgBytes, msgProto); err != nil {
		logger.Error("error decoding message", "err", err)
		_ = r.reporter.Report(behavior.BadMessage(src.ID(), err.Error()))
		return
	}

	if err := msgProto.Validate(); err != nil {
		logger.Error("peer sent us an invalid msg", "msg", msgProto, "err", err)
		_ = r.reporter.Report(behavior.BadMessage(src.ID(), err.Error()))
		return
	}

	r.logger.Debug("received", "msg", msgProto)

	switch msg := msgProto.Sum.(type) {
	case *bcproto.Message_StatusRequest:
		if err := r.io.sendStatusResponse(r.store.Base(), r.store.Height(), src); err != nil {
			logger.Error("Could not send status message to src peer")
		}

	case *bcproto.Message_BlockRequest:
		block := r.store.LoadBlock(msg.BlockRequest.Height)
		if block != nil {
			if err := r.io.sendBlockToPeer(block, src); err != nil {
				logger.Error("Could not send block message to src peer", "err", err)
			}
		} else {
			logger.Info("peer asking for a block we don't have", "height", msg.BlockRequest.Height)
			if err := r.io.sendBlockNotFound(msg.BlockRequest.Height, src); err != nil {
				logger.Error("Couldn't send block not found msg", "err", err)
			}
		}

	case *bcproto.Message_StatusResponse:
		r.mtx.RLock()
		if r.events != nil {
			r.events <- bcStatusResponse{
				peerID: src.ID(),
				base:   msg.StatusResponse.Base,
				height: msg.StatusResponse.Height,
			}
		}
		r.mtx.RUnlock()

	case *bcproto.Message_BlockResponse:
		bi, err := types.BlockFromProto(msg.BlockResponse.Block)
		if err != nil {
			logger.Error("error transitioning block from protobuf", "err", err)
			_ = r.reporter.Report(behavior.BadMessage(src.ID(), err.Error()))
			return
		}
		r.mtx.RLock()
		if r.events != nil {
			r.events <- bcBlockResponse{
				peerID: src.ID(),
				block:  bi,
				size:   int64(len(msgBytes)),
				time:   time.Now(),
			}
		}
		r.mtx.RUnlock()

	case *bcproto.Message_NoBlockResponse:
		r.mtx.RLock()
		if r.events != nil {
			r.events <- bcNoBlockResponse{
				peerID: src.ID(),
				height: msg.NoBlockResponse.Height,
				time:   time.Now(),
			}
		}
		r.mtx.RUnlock()
	}
}

// AddPeer implements Reactor interface
func (r *BlockchainReactor) AddPeer(peer p2p.Peer) {
	err := r.io.sendStatusResponse(r.store.Base(), r.store.Height(), peer)
	if err != nil {
		r.logger.Error("could not send our status to the new peer", "peer", peer.ID, "err", err)
	}

	err = r.io.sendStatusRequest(peer)
	if err != nil {
		r.logger.Error("could not send status request to the new peer", "peer", peer.ID, "err", err)
	}

	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if r.events != nil {
		r.events <- bcAddNewPeer{peerID: peer.ID()}
	}
}

// RemovePeer implements Reactor interface.
func (r *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if r.events != nil {
		r.events <- bcRemovePeer{
			peerID: peer.ID(),
			reason: reason,
		}
	}
}

// GetChannels implements Reactor
func (r *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            5,
			SendQueueCapacity:   2000,
			RecvBufferCapacity:  1024,
			RecvMessageCapacity: blocksync.MaxMsgSize,
		},
	}
}

func (r *BlockchainReactor) GetMaxPeerBlockHeight() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.maxPeerHeight
}

func (r *BlockchainReactor) GetTotalSyncedTime() time.Duration {
	if !r.blockSync.IsSet() || r.syncStartTime.IsZero() {
		return time.Duration(0)
	}
	return time.Since(r.syncStartTime)
}

func (r *BlockchainReactor) GetRemainingSyncTime() time.Duration {
	if !r.blockSync.IsSet() {
		return time.Duration(0)
	}

	r.mtx.RLock()
	defer r.mtx.RUnlock()

	targetSyncs := r.maxPeerHeight - r.syncStartHeight
	currentSyncs := r.syncHeight - r.syncStartHeight + 1
	if currentSyncs < 0 || r.lastSyncRate < 0.001 {
		return time.Duration(0)
	}

	remain := float64(targetSyncs-currentSyncs) / r.lastSyncRate

	return time.Duration(int64(remain * float64(time.Second)))
}
