package v2

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/behaviour"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-------------------------------------

type bcBlockRequestMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcBlockRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *bcBlockRequestMessage) String() string {
	return fmt.Sprintf("[bcBlockRequestMessage %v]", m.Height)
}

type bcNoBlockResponseMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcNoBlockResponseMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *bcNoBlockResponseMessage) String() string {
	return fmt.Sprintf("[bcNoBlockResponseMessage %d]", m.Height)
}

//-------------------------------------

type bcBlockResponseMessage struct {
	Block *types.Block
}

// ValidateBasic performs basic validation.
func (m *bcBlockResponseMessage) ValidateBasic() error {
	if m.Block == nil {
		return errors.New("block response message has nil block")
	}

	return m.Block.ValidateBasic()
}

func (m *bcBlockResponseMessage) String() string {
	return fmt.Sprintf("[bcBlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type bcStatusRequestMessage struct {
	Height int64
	Base   int64
}

// ValidateBasic performs basic validation.
func (m *bcStatusRequestMessage) ValidateBasic() error {
	if m.Base < 0 {
		return errors.New("negative Base")
	}
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Base > m.Height {
		return fmt.Errorf("base %v cannot be greater than height %v", m.Base, m.Height)
	}
	return nil
}

func (m *bcStatusRequestMessage) String() string {
	return fmt.Sprintf("[bcStatusRequestMessage %v:%v]", m.Base, m.Height)
}

//-------------------------------------

type bcStatusResponseMessage struct {
	Height int64
	Base   int64
}

// ValidateBasic performs basic validation.
func (m *bcStatusResponseMessage) ValidateBasic() error {
	if m.Base < 0 {
		return errors.New("negative Base")
	}
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Base > m.Height {
		return fmt.Errorf("base %v cannot be greater than height %v", m.Base, m.Height)
	}
	return nil
}

func (m *bcStatusResponseMessage) String() string {
	return fmt.Sprintf("[bcStatusResponseMessage %v:%v]", m.Base, m.Height)
}

type blockStore interface {
	LoadBlock(height int64) *types.Block
	SaveBlock(*types.Block, *types.PartSet, *types.Commit)
	Base() int64
	Height() int64
}

// BlockchainReactor handles fast sync protocol.
type BlockchainReactor struct {
	p2p.BaseReactor

	fastSync  bool       // if true, enable fast sync on start
	events    chan Event // XXX: Rename eventsFromPeers
	scheduler *Routine
	processor *Routine
	logger    log.Logger

	mtx           sync.RWMutex
	maxPeerHeight int64
	syncHeight    int64

	reporter behaviour.Reporter
	io       iIO
	store    blockStore
}

//nolint:unused,deadcode
type blockVerifier interface {
	VerifyCommit(chainID string, blockID types.BlockID, height int64, commit *types.Commit) error
}

//nolint:deadcode
type blockApplier interface {
	ApplyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, int64, error)
}

// XXX: unify naming in this package around tmState
// XXX: V1 stores a copy of state as initialState, which is never mutated. Is that nessesary?
func newReactor(state state.State, store blockStore, reporter behaviour.Reporter,
	blockApplier blockApplier, bufferSize int, fastSync bool) *BlockchainReactor {
	scheduler := newScheduler(state.LastBlockHeight, time.Now())
	pContext := newProcessorContext(store, blockApplier, state)
	// TODO: Fix naming to just newProcesssor
	// newPcState requires a processorContext
	processor := newPcState(pContext)

	return &BlockchainReactor{
		events:    make(chan Event, bufferSize),
		scheduler: newRoutine("scheduler", scheduler.handle, bufferSize),
		processor: newRoutine("processor", processor.handle, bufferSize),
		store:     store,
		reporter:  reporter,
		logger:    log.NewNopLogger(),
		fastSync:  fastSync,
	}
}

// NewBlockchainReactor creates a new reactor instance.
func NewBlockchainReactor(
	state state.State,
	blockApplier blockApplier,
	store blockStore,
	fastSync bool) *BlockchainReactor {
	reporter := behaviour.NewMockReporter()
	return newReactor(state, store, reporter, blockApplier, 1000, fastSync)
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
	r.reporter = behaviour.NewSwitchReporter(r.BaseReactor.Switch)
	if r.fastSync {
		go r.scheduler.start()
		go r.processor.start()
		go r.demux()
	}
	return nil
}

// reactor generated ticker events:
// ticker for cleaning peers
type rTryPrunePeer struct {
	priorityHigh
	time time.Time
}

func (e rTryPrunePeer) String() string {
	return fmt.Sprintf(": %v", e.time)
}

// ticker event for scheduling block requests
type rTrySchedule struct {
	priorityHigh
	time time.Time
}

func (e rTrySchedule) String() string {
	return fmt.Sprintf(": %v", e.time)
}

// ticker for block processing
type rProcessBlock struct {
	priorityNormal
}

// reactor generated events based on blockchain related messages from peers:
// blockResponse message received from a peer
type bcBlockResponse struct {
	priorityNormal
	time   time.Time
	peerID p2p.ID
	size   int64
	block  *types.Block
}

// blockNoResponse message received from a peer
type bcNoBlockResponse struct {
	priorityNormal
	time   time.Time
	peerID p2p.ID
	height int64
}

// statusResponse message received from a peer
type bcStatusResponse struct {
	priorityNormal
	time   time.Time
	peerID p2p.ID
	base   int64
	height int64
}

// new peer is connected
type bcAddNewPeer struct {
	priorityNormal
	peerID p2p.ID
}

// existing peer is removed
type bcRemovePeer struct {
	priorityHigh
	peerID p2p.ID
	reason interface{}
}

func (r *BlockchainReactor) demux() {
	var lastRate = 0.0
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
			r.io.broadcastStatusRequest(r.store.Base(), r.SyncHeight())

		// Events from peers. Closing the channel signals event loop termination.
		case event, ok := <-r.events:
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
				r.logger.Error("Received unknown event", "event", fmt.Sprintf("%T", event))
			}

		// Incremental events form scheduler
		case event := <-r.scheduler.next():
			switch event := event.(type) {
			case scBlockReceived:
				r.processor.send(event)
			case scPeerError:
				r.processor.send(event)
				r.reporter.Report(behaviour.BadMessage(event.peerID, "scPeerError"))
			case scBlockRequest:
				r.io.sendBlockRequest(event.peerID, event.height)
			case scFinishedEv:
				r.processor.send(event)
				r.scheduler.stop()
			case noOpEvent:
			default:
				r.logger.Error("Received unknown scheduler event", "event", fmt.Sprintf("%T", event))
			}

		// Incremental events from processor
		case event := <-r.processor.next():
			switch event := event.(type) {
			case pcBlockProcessed:
				r.setSyncHeight(event.height)
				if r.syncHeight%100 == 0 {
					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					r.logger.Info("Fast Syncc Rate", "height", r.syncHeight,
						"max_peer_height", r.maxPeerHeight, "blocks/s", lastRate)
					lastHundred = time.Now()
				}
				r.scheduler.send(event)
			case pcBlockVerificationFailure:
				r.scheduler.send(event)
			case pcFinished:
				r.io.trySwitchToConsensus(event.tmState, event.blocksSynced)
				r.processor.stop()
			case noOpEvent:
			default:
				r.logger.Error("Received unknown processor event", "event", fmt.Sprintf("%T", event))
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

	r.scheduler.stop()
	r.processor.stop()
	close(r.events)

	r.logger.Info("reactor stopped")
	return nil
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

// RegisterBlockchainMessages registers the fast sync messages for amino encoding.
func RegisterBlockchainMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*BlockchainMessage)(nil), nil)
	cdc.RegisterConcrete(&bcBlockRequestMessage{}, "tendermint/blockchain/BlockRequest", nil)
	cdc.RegisterConcrete(&bcBlockResponseMessage{}, "tendermint/blockchain/BlockResponse", nil)
	cdc.RegisterConcrete(&bcNoBlockResponseMessage{}, "tendermint/blockchain/NoBlockResponse", nil)
	cdc.RegisterConcrete(&bcStatusResponseMessage{}, "tendermint/blockchain/StatusResponse", nil)
	cdc.RegisterConcrete(&bcStatusRequestMessage{}, "tendermint/blockchain/StatusRequest", nil)
}

func decodeMsg(bz []byte) (msg BlockchainMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

// Receive implements Reactor by handling different message types.
func (r *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		r.logger.Error("error decoding message",
			"src", src.ID(), "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		_ = r.reporter.Report(behaviour.BadMessage(src.ID(), err.Error()))
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		r.logger.Error("peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		_ = r.reporter.Report(behaviour.BadMessage(src.ID(), err.Error()))
		return
	}

	r.logger.Debug("Receive", "src", src.ID(), "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcStatusRequestMessage:
		if err := r.io.sendStatusResponse(r.store.Height(), src.ID()); err != nil {
			r.logger.Error("Could not send status message to peer", "src", src)
		}

	case *bcBlockRequestMessage:
		block := r.store.LoadBlock(msg.Height)
		if block != nil {
			if err = r.io.sendBlockToPeer(block, src.ID()); err != nil {
				r.logger.Error("Could not send block message to peer: ", err)
			}
		} else {
			r.logger.Info("peer asking for a block we don't have", "src", src, "height", msg.Height)
			peerID := src.ID()
			if err = r.io.sendBlockNotFound(msg.Height, peerID); err != nil {
				r.logger.Error("Couldn't send block not found: ", err)
			}
		}

	case *bcStatusResponseMessage:
		r.events <- bcStatusResponse{peerID: src.ID(), base: msg.Base, height: msg.Height}

	case *bcBlockResponseMessage:
		r.events <- bcBlockResponse{
			peerID: src.ID(),
			block:  msg.Block,
			size:   int64(len(msgBytes)),
			time:   time.Now(),
		}

	case *bcNoBlockResponseMessage:
		r.events <- bcNoBlockResponse{peerID: src.ID(), height: msg.Height, time: time.Now()}
	}
}

// AddPeer implements Reactor interface
func (r *BlockchainReactor) AddPeer(peer p2p.Peer) {
	err := r.io.sendStatusResponse(r.store.Height(), peer.ID())
	if err != nil {
		r.logger.Error("Could not send status message to peer new", "src", peer.ID, "height", r.SyncHeight())
	}
	r.events <- bcAddNewPeer{peerID: peer.ID()}
}

// RemovePeer implements Reactor interface.
func (r *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	event := bcRemovePeer{
		peerID: peer.ID(),
		reason: reason,
	}
	r.events <- event
}

// GetChannels implements Reactor
func (r *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            10,
			SendQueueCapacity:   2000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}
