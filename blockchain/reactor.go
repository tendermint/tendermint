package blockchain

import (
	"bytes"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	BlockchainChannel      = byte(0x40)
	defaultChannelCapacity = 100
	defaultSleepIntervalMS = 500
)

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	sw         *p2p.Switch
	store      *BlockStore
	pool       *BlockPool
	requestsCh chan BlockRequest
	timeoutsCh chan string
	lastBlock  *types.Block
	quit       chan struct{}
	started    uint32
	stopped    uint32
}

func NewBlockchainReactor(store *BlockStore) *BlockchainReactor {
	requestsCh := make(chan BlockRequest, defaultChannelCapacity)
	timeoutsCh := make(chan string, defaultChannelCapacity)
	pool := NewBlockPool(
		store.Height()+1,
		requestsCh,
		timeoutsCh,
	)
	bcR := &BlockchainReactor{
		store:      store,
		pool:       pool,
		requestsCh: requestsCh,
		timeoutsCh: timeoutsCh,
		quit:       make(chan struct{}),
		started:    0,
		stopped:    0,
	}
	return bcR
}

// Implements Reactor
func (bcR *BlockchainReactor) Start(sw *p2p.Switch) {
	if atomic.CompareAndSwapUint32(&bcR.started, 0, 1) {
		log.Info("Starting BlockchainReactor")
		bcR.sw = sw
		bcR.pool.Start()
		go bcR.poolRoutine()
	}
}

// Implements Reactor
func (bcR *BlockchainReactor) Stop() {
	if atomic.CompareAndSwapUint32(&bcR.stopped, 0, 1) {
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
			SendQueueCapacity: 20, // Queue 20 blocks to send to a peer.
		},
	}
}

// Implements Reactor
func (bcR *BlockchainReactor) AddPeer(peer *p2p.Peer) {
	// Send peer our state.
	peer.Send(BlockchainChannel, PeerStatusMessage{bcR.store.Height()})
}

// Implements Reactor
func (bcR *BlockchainReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	// Remove peer from the pool.
	bcR.pool.RemovePeer(peer.Key)
}

// Implements Reactor
func (bcR *BlockchainReactor) Receive(chId byte, src *p2p.Peer, msgBytes []byte) {
	_, msg_, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}
	log.Info("BlockchainReactor received message", "msg", msg_)

	switch msg := msg_.(type) {
	case BlockRequestMessage:
		log.Debug("Got BlockRequest", "msg", msg)
		// Got a request for a block. Respond with block if we have it.
		block := bcR.store.LoadBlock(msg.Height)
		if block != nil {
			msg := BlockResponseMessage{Block: block}
			queued := src.TrySend(BlockchainChannel, msg)
			if !queued {
				// queue is full, just ignore.
			}
		} else {
			// TODO peer is asking for things we don't have.
		}
	case BlockResponseMessage:
		log.Debug("Got BlockResponse", "msg", msg)
		// Got a block.
		bcR.pool.AddBlock(msg.Block, src.Key)
	case PeerStatusMessage:
		log.Debug("Got PeerStatus", "msg", msg)
		// Got a peer status.
		bcR.pool.SetPeerHeight(src.Key, msg.Height)
	default:
		// Ignore unknown message
	}
}

func (bcR *BlockchainReactor) poolRoutine() {
FOR_LOOP:
	for {
		select {
		case request := <-bcR.requestsCh: // chan BlockRequest
			peer := bcR.sw.Peers().Get(request.PeerId)
			if peer == nil {
				// We can't fulfill the request.
				continue FOR_LOOP
			}
			msg := BlockRequestMessage{request.Height}
			queued := peer.TrySend(BlockchainChannel, msg)
			if !queued {
				// We couldn't queue the request.
				time.Sleep(defaultSleepIntervalMS * time.Millisecond)
				continue FOR_LOOP
			}
		case peerId := <-bcR.timeoutsCh: // chan string
			// Peer timed out.
			peer := bcR.sw.Peers().Get(peerId)
			bcR.sw.StopPeerForError(peer, errors.New("BlockchainReactor Timeout"))
		case <-bcR.quit:
			break FOR_LOOP
		}
	}
}

func (bcR *BlockchainReactor) BroadcastStatus() error {
	bcR.sw.Broadcast(BlockchainChannel, PeerStatusMessage{bcR.store.Height()})
	return nil
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown       = byte(0x00)
	msgTypeBlockRequest  = byte(0x10)
	msgTypeBlockResponse = byte(0x11)
	msgTypePeerStatus    = byte(0x20)
)

// TODO: check for unnecessary extra bytes at the end.
func DecodeMessage(bz []byte) (msgType byte, msg interface{}, err error) {
	n := new(int64)
	msgType = bz[0]
	r := bytes.NewReader(bz)
	switch msgType {
	case msgTypeBlockRequest:
		msg = binary.ReadBinary(BlockRequestMessage{}, r, n, &err)
	case msgTypeBlockResponse:
		msg = binary.ReadBinary(BlockResponseMessage{}, r, n, &err)
	case msgTypePeerStatus:
		msg = binary.ReadBinary(PeerStatusMessage{}, r, n, &err)
	default:
		msg = nil
	}
	return
}

//-------------------------------------

type BlockRequestMessage struct {
	Height uint
}

func (m BlockRequestMessage) TypeByte() byte { return msgTypeBlockRequest }

func (m BlockRequestMessage) String() string {
	return fmt.Sprintf("[BlockRequestMessage %v]", m.Height)
}

//-------------------------------------

type BlockResponseMessage struct {
	Block *types.Block
}

func (m BlockResponseMessage) TypeByte() byte { return msgTypeBlockResponse }

func (m BlockResponseMessage) String() string {
	return fmt.Sprintf("[BlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type PeerStatusMessage struct {
	Height uint
}

func (m PeerStatusMessage) TypeByte() byte { return msgTypePeerStatus }

func (m PeerStatusMessage) String() string {
	return fmt.Sprintf("[PeerStatusMessage %v]", m.Height)
}
