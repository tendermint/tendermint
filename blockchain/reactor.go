package blockchain

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	BlockchainChannel      = byte(0x40)
	defaultChannelCapacity = 100
	defaultSleepIntervalMS = 500
	trySyncIntervalMS      = 100
	// stop syncing when last block's time is
	// within this much of the system time.
	// stopSyncingDurationMinutes = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10
	// check if we should switch to consensus reactor
	switchToConsensusIntervalSeconds = 10
)

type consensusReactor interface {
	SetSyncing(bool)
	ResetToState(*sm.State)
}

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	sw         *p2p.Switch
	state      *sm.State
	store      *BlockStore
	pool       *BlockPool
	sync       bool
	requestsCh chan BlockRequest
	timeoutsCh chan string
	lastBlock  *types.Block
	quit       chan struct{}
	running    uint32

	evsw events.Fireable
}

func NewBlockchainReactor(state *sm.State, store *BlockStore, sync bool) *BlockchainReactor {
	if state.LastBlockHeight != store.Height() &&
		state.LastBlockHeight != store.Height()-1 { // XXX double check this logic.
		panic(Fmt("state (%v) and store (%v) height mismatch", state.LastBlockHeight, store.Height()))
	}
	requestsCh := make(chan BlockRequest, defaultChannelCapacity)
	timeoutsCh := make(chan string, defaultChannelCapacity)
	pool := NewBlockPool(
		store.Height()+1,
		requestsCh,
		timeoutsCh,
	)
	bcR := &BlockchainReactor{
		state:      state,
		store:      store,
		pool:       pool,
		sync:       sync,
		requestsCh: requestsCh,
		timeoutsCh: timeoutsCh,
		quit:       make(chan struct{}),
		running:    uint32(0),
	}
	return bcR
}

// Implements Reactor
func (bcR *BlockchainReactor) Start(sw *p2p.Switch) {
	if atomic.CompareAndSwapUint32(&bcR.running, 0, 1) {
		log.Info("Starting BlockchainReactor")
		bcR.sw = sw
		if bcR.sync {
			bcR.pool.Start()
			go bcR.poolRoutine()
		}
	}
}

// Implements Reactor
func (bcR *BlockchainReactor) Stop() {
	if atomic.CompareAndSwapUint32(&bcR.running, 1, 0) {
		log.Info("Stopping BlockchainReactor")
		close(bcR.quit)
		bcR.pool.Stop()
	}
}

// Implements Reactor
func (bcR *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			Id:                BlockchainChannel,
			Priority:          5,
			SendQueueCapacity: 100,
		},
	}
}

// Implements Reactor
func (bcR *BlockchainReactor) AddPeer(peer *p2p.Peer) {
	// Send peer our state.
	peer.Send(BlockchainChannel, &bcStatusResponseMessage{bcR.store.Height()})
}

// Implements Reactor
func (bcR *BlockchainReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	// Remove peer from the pool.
	bcR.pool.RemovePeer(peer.Key)
}

// Implements Reactor
func (bcR *BlockchainReactor) Receive(chId byte, src *p2p.Peer, msgBytes []byte) {
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}

	log.Info("Received message", "msg", msg)

	switch msg := msg.(type) {
	case *bcBlockRequestMessage:
		// Got a request for a block. Respond with block if we have it.
		block := bcR.store.LoadBlock(msg.Height)
		if block != nil {
			msg := &bcBlockResponseMessage{Block: block}
			queued := src.TrySend(BlockchainChannel, msg)
			if !queued {
				// queue is full, just ignore.
			}
		} else {
			// TODO peer is asking for things we don't have.
		}
	case *bcBlockResponseMessage:
		// Got a block.
		bcR.pool.AddBlock(msg.Block, src.Key)
	case *bcStatusRequestMessage:
		// Send peer our state.
		queued := src.TrySend(BlockchainChannel, &bcStatusResponseMessage{bcR.store.Height()})
		if !queued {
			// sorry
		}
	case *bcStatusResponseMessage:
		// Got a peer status. Unverified.
		bcR.pool.SetPeerHeight(src.Key, msg.Height)
	default:
		log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
// (Except for the SYNC_LOOP, which is the primary purpose and must be synchronous.)
func (bcR *BlockchainReactor) poolRoutine() {

	trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
	switchToConsensusTicker := time.NewTicker(switchToConsensusIntervalSeconds * time.Second)

FOR_LOOP:
	for {
		select {
		case request := <-bcR.requestsCh: // chan BlockRequest
			peer := bcR.sw.Peers().Get(request.PeerId)
			if peer == nil {
				// We can't assign the request.
				continue FOR_LOOP
			}
			msg := &bcBlockRequestMessage{request.Height}
			queued := peer.TrySend(BlockchainChannel, msg)
			if !queued {
				// We couldn't make the request, send-queue full.
				// The pool handles retries, so just let it go.
				continue FOR_LOOP
			}
		case peerId := <-bcR.timeoutsCh: // chan string
			// Peer timed out.
			peer := bcR.sw.Peers().Get(peerId)
			if peer != nil {
				bcR.sw.StopPeerForError(peer, errors.New("BlockchainReactor Timeout"))
			}
		case _ = <-statusUpdateTicker.C:
			// ask for status updates
			go bcR.BroadcastStatusRequest()
		case _ = <-switchToConsensusTicker.C:
			// not thread safe access for numUnassigned and numPending but should be fine
			// TODO make threadsafe and use exposed functions
			outbound, inbound, _ := bcR.sw.NumPeers()
			log.Debug("Consensus ticker", "numUnassigned", bcR.pool.numUnassigned, "numPending", bcR.pool.numPending,
				"total", len(bcR.pool.requests), "outbound", outbound, "inbound", inbound)
			// NOTE: this condition is very strict right now. may need to weaken
			// If all `maxPendingRequests` requests are unassigned
			// and we have some peers (say >= 3), then we're caught up
			maxPending := bcR.pool.numPending == maxPendingRequests
			allUnassigned := bcR.pool.numPending == bcR.pool.numUnassigned
			enoughPeers := outbound+inbound >= 3
			if maxPending && allUnassigned && enoughPeers {
				log.Info("Time to switch to consensus reactor!", "height", bcR.pool.height)
				bcR.pool.Stop()
				stateDB := dbm.GetDB("state")
				state := sm.LoadState(stateDB)

				bcR.sw.Reactor("CONSENSUS").(consensusReactor).ResetToState(state)
				bcR.sw.Reactor("CONSENSUS").(consensusReactor).SetSyncing(false)

				break FOR_LOOP
			}
		case _ = <-trySyncTicker.C: // chan time
			// This loop can be slow as long as it's doing syncing work.
		SYNC_LOOP:
			for i := 0; i < 10; i++ {
				// See if there are any blocks to sync.
				first, second := bcR.pool.PeekTwoBlocks()
				//log.Debug("TrySync peeked", "first", first, "second", second)
				if first == nil || second == nil {
					// We need both to sync the first block.
					break SYNC_LOOP
				}
				firstParts := first.MakePartSet()
				firstPartsHeader := firstParts.Header()
				// Finally, verify the first block using the second's validation.
				err := bcR.state.BondedValidators.VerifyValidation(
					first.Hash(), firstPartsHeader, first.Height, second.Validation)
				if err != nil {
					log.Debug("error in validation", "error", err)
					bcR.pool.RedoRequest(first.Height)
					break SYNC_LOOP
				} else {
					bcR.pool.PopRequest()
					err := sm.ExecBlock(bcR.state, first, firstPartsHeader)
					if err != nil {
						// TODO This is bad, are we zombie?
						panic(Fmt("Failed to process committed block: %v", err))
					}
					bcR.store.SaveBlock(first, firstParts, second.Validation)
					bcR.state.Save()
				}
			}
			continue FOR_LOOP
		case <-bcR.quit:
			break FOR_LOOP
		}
	}
}

func (bcR *BlockchainReactor) BroadcastStatusResponse() error {
	bcR.sw.Broadcast(BlockchainChannel, &bcStatusResponseMessage{bcR.store.Height()})
	return nil
}

func (bcR *BlockchainReactor) BroadcastStatusRequest() error {
	bcR.sw.Broadcast(BlockchainChannel, &bcStatusRequestMessage{bcR.store.Height()})
	return nil
}

// implements events.Eventable
func (bcR *BlockchainReactor) SetFireable(evsw events.Fireable) {
	bcR.evsw = evsw
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeBlockRequest   = byte(0x10)
	msgTypeBlockResponse  = byte(0x11)
	msgTypeStatusResponse = byte(0x20)
	msgTypeStatusRequest  = byte(0x21)
)

type BlockchainMessage interface{}

var _ = binary.RegisterInterface(
	struct{ BlockchainMessage }{},
	binary.ConcreteType{&bcBlockRequestMessage{}, msgTypeBlockRequest},
	binary.ConcreteType{&bcBlockResponseMessage{}, msgTypeBlockResponse},
	binary.ConcreteType{&bcStatusResponseMessage{}, msgTypeStatusResponse},
	binary.ConcreteType{&bcStatusRequestMessage{}, msgTypeStatusRequest},
)

func DecodeMessage(bz []byte) (msgType byte, msg BlockchainMessage, err error) {
	msgType = bz[0]
	n := new(int64)
	r := bytes.NewReader(bz)
	msg = binary.ReadBinary(struct{ BlockchainMessage }{}, r, n, &err).(struct{ BlockchainMessage }).BlockchainMessage
	return
}

//-------------------------------------

type bcBlockRequestMessage struct {
	Height uint
}

func (m *bcBlockRequestMessage) String() string {
	return fmt.Sprintf("[bcBlockRequestMessage %v]", m.Height)
}

//-------------------------------------

type bcBlockResponseMessage struct {
	Block *types.Block
}

func (m *bcBlockResponseMessage) String() string {
	return fmt.Sprintf("[bcBlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type bcStatusRequestMessage struct {
	Height uint
}

func (m *bcStatusRequestMessage) String() string {
	return fmt.Sprintf("[bcStatusRequestMessage %v]", m.Height)
}

//-------------------------------------

type bcStatusResponseMessage struct {
	Height uint
}

func (m *bcStatusResponseMessage) String() string {
	return fmt.Sprintf("[bcStatusResponseMessage %v]", m.Height)
}
