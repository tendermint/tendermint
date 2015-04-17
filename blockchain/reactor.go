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
	stopSyncingDurationMinutes = 10
)

type stateResetter interface {
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
		bcR.pool.Start()
		if bcR.sync {
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
	peer.Send(BlockchainChannel, &bcPeerStatusMessage{bcR.store.Height()})
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
	case *bcPeerStatusMessage:
		// Got a peer status.
		bcR.pool.SetPeerHeight(src.Key, msg.Height)
	default:
		log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
func (bcR *BlockchainReactor) poolRoutine() {

	trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)

FOR_LOOP:
	for {
		select {
		case request := <-bcR.requestsCh: // chan BlockRequest
			peer := bcR.sw.Peers().Get(request.PeerId)
			if peer == nil {
				// We can't fulfill the request.
				continue FOR_LOOP
			}
			msg := &bcBlockRequestMessage{request.Height}
			queued := peer.TrySend(BlockchainChannel, msg)
			if !queued {
				// We couldn't queue the request.
				time.Sleep(defaultSleepIntervalMS * time.Millisecond)
				continue FOR_LOOP
			}
		case peerId := <-bcR.timeoutsCh: // chan string
			// Peer timed out.
			peer := bcR.sw.Peers().Get(peerId)
			if peer != nil {
				bcR.sw.StopPeerForError(peer, errors.New("BlockchainReactor Timeout"))
			}
		case _ = <-trySyncTicker.C: // chan time
			//var lastValidatedBlock *types.Block
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
					//lastValidatedBlock = first
				}
			}
			/*
				// We're done syncing for now (will do again shortly)
				// See if we want to stop syncing and turn on the
				// consensus reactor.
				// TODO: use other heuristics too besides blocktime.
				// It's not a security concern, as it only needs to happen
				// upon node sync, and there's also a second (slower)
				// method of syncing in the consensus reactor.

				if lastValidatedBlock != nil && time.Now().Sub(lastValidatedBlock.Time) < stopSyncingDurationMinutes*time.Minute {
					go func() {
						log.Info("Stopping blockpool syncing, turning on consensus...")
						trySyncTicker.Stop() // Just stop the block requests.  Still serve blocks to others.
						conR := bcR.sw.Reactor("CONSENSUS")
						conR.(stateResetter).ResetToState(bcR.state)
						conR.Start(bcR.sw)
						for _, peer := range bcR.sw.Peers().List() {
							conR.AddPeer(peer)
						}
					}()
					break FOR_LOOP
				}
			*/
			continue FOR_LOOP
		case <-bcR.quit:
			break FOR_LOOP
		}
	}
}

func (bcR *BlockchainReactor) BroadcastStatus() error {
	bcR.sw.Broadcast(BlockchainChannel, &bcPeerStatusMessage{bcR.store.Height()})
	return nil
}

// implements events.Eventable
func (bcR *BlockchainReactor) SetFireable(evsw events.Fireable) {
	bcR.evsw = evsw
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeBlockRequest  = byte(0x10)
	msgTypeBlockResponse = byte(0x11)
	msgTypePeerStatus    = byte(0x20)
)

type BlockchainMessage interface{}

var _ = binary.RegisterInterface(
	struct{ BlockchainMessage }{},
	binary.ConcreteType{&bcBlockRequestMessage{}, msgTypeBlockRequest},
	binary.ConcreteType{&bcBlockResponseMessage{}, msgTypeBlockResponse},
	binary.ConcreteType{&bcPeerStatusMessage{}, msgTypePeerStatus},
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

type bcPeerStatusMessage struct {
	Height uint
}

func (m *bcPeerStatusMessage) String() string {
	return fmt.Sprintf("[bcPeerStatusMessage %v]", m.Height)
}
