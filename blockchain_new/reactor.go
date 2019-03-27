package blockchain_new

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)

	maxTotalMessages = 1000

	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(sm.State, int)
}

type peerError struct {
	err    error
	peerID p2p.ID
}

func (e peerError) Error() string {
	return fmt.Sprintf("error with peer %v: %s", e.peerID, e.err.Error())
}

type bReactorMsgFromFSM uint

// message types
const (
	// message type events
	errorMsg = iota + 1
	//blockRequestMsg
	//statusRequestMsg
)

type msgFromFSM struct {
	msgType bReactorMsgFromFSM
	error   peerError
}

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	p2p.BaseReactor

	// immutable
	initialState sm.State
	state        sm.State

	blockExec *sm.BlockExecutor
	store     *BlockStore

	fastSync bool

	fsm          *bReactorFSM
	blocksSynced int
	lastHundred  time.Time
	lastRate     float64

	msgFromFSMCh chan msgFromFSM
}

// NewBlockchainReactor returns new reactor instance.
func NewBlockchainReactor(state sm.State, blockExec *sm.BlockExecutor, store *BlockStore,
	fastSync bool) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	bcR := &BlockchainReactor{
		initialState: state,
		state:        state,
		blockExec:    blockExec,
		fastSync:     fastSync,
		store:        store,
	}
	fsm := NewFSM(store.Height()+1, bcR)
	bcR.fsm = fsm
	bcR.BaseReactor = *p2p.NewBaseReactor("BlockchainReactor", bcR)
	return bcR
}

// SetLogger implements cmn.Service by setting the logger on reactor and pool.
func (bcR *BlockchainReactor) SetLogger(l log.Logger) {
	bcR.BaseService.Logger = l
	bcR.fsm.setLogger(l)
}

// OnStart implements cmn.Service.
func (bcR *BlockchainReactor) OnStart() error {
	if bcR.fastSync {
		bcR.fsm.start()
		go bcR.poolRoutine()
	}
	return nil
}

// OnStop implements cmn.Service.
func (bcR *BlockchainReactor) OnStop() {
	bcR.fsm.stop()
}

// GetChannels implements Reactor
func (bcR *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            10,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our state to peer.
func (bcR *BlockchainReactor) AddPeer(peer p2p.Peer) {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{bcR.store.Height()})
	if !peer.Send(BlockchainChannel, msgBytes) {
		// doing nothing, will try later in `poolRoutine`
	}
	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.SetPeerHeight
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	bcR.fsm.RemovePeer(peer.ID())
}

// respondToPeer loads a block and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
// According to the Tendermint spec, if all nodes are honest,
// no node should be requesting for a block that's non-existent.
func (bcR *BlockchainReactor) respondToPeer(msg *bcBlockRequestMessage,
	src p2p.Peer) (queued bool) {

	block := bcR.store.LoadBlock(msg.Height)
	if block != nil {
		msgBytes := cdc.MustMarshalBinaryBare(&bcBlockResponseMessage{Block: block})
		return src.TrySend(BlockchainChannel, msgBytes)
	}

	bcR.Logger.Info("peer asking for a block we don't have", "src", src, "height", msg.Height)

	msgBytes := cdc.MustMarshalBinaryBare(&bcNoBlockResponseMessage{Height: msg.Height})
	return src.TrySend(BlockchainChannel, msgBytes)
}

// Receive implements Reactor by handling 4 types of messages (look below).
func (bcR *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		bcR.Logger.Error("error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		bcR.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		bcR.Logger.Error("peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		bcR.Switch.StopPeerForError(src, err)
		return
	}

	bcR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcBlockRequestMessage:
		if queued := bcR.respondToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
		}
	case *bcBlockResponseMessage:
		msgData := bReactorMessageData{
			event: blockResponseEv,
			data: bReactorEventData{
				peerId: src.ID(),
				block:  msg.Block,
				length: len(msgBytes),
			},
		}
		sendMessageToFSM(bcR.fsm, msgData)

	case *bcStatusRequestMessage:
		// Send peer our state.
		msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{bcR.store.Height()})
		queued := src.TrySend(BlockchainChannel, msgBytes)
		if !queued {
			// sorry
		}
	case *bcStatusResponseMessage:
		// Got a peer status. Unverified.
		msgData := bReactorMessageData{
			event: statusResponseEv,
			data: bReactorEventData{
				peerId: src.ID(),
				height: msg.Height,
				length: len(msgBytes),
			},
		}
		sendMessageToFSM(bcR.fsm, msgData)

	default:
		bcR.Logger.Error(fmt.Sprintf("unknown message type %v", reflect.TypeOf(msg)))
	}
}

func (bcR *BlockchainReactor) processBlocks(first *types.Block, second *types.Block) error {

	chainID := bcR.initialState.ChainID

	firstParts := first.MakePartSet(types.BlockPartSizeBytes)
	firstPartsHeader := firstParts.Header()
	firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}
	// Finally, verify the first block using the second's commit
	// NOTE: we can probably make this more efficient, but note that calling
	// first.Hash() doesn't verify the tx contents, so MakePartSet() is
	// currently necessary.
	err := bcR.state.Validators.VerifyCommit(
		chainID, firstID, first.Height, second.LastCommit)

	if err != nil {
		bcR.Logger.Error("error in validation", "err", err, first.Height, second.Height)
		return errBlockVerificationFailure
	}

	bcR.store.SaveBlock(first, firstParts, second.LastCommit)

	// get the hash without persisting the state
	bcR.state, err = bcR.blockExec.ApplyBlock(bcR.state, firstID, first)
	if err != nil {
		// TODO This is bad, are we zombie?
		panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
	}
	bcR.blocksSynced++

	if bcR.blocksSynced%100 == 0 {
		bcR.lastRate = 0.9*bcR.lastRate + 0.1*(100/time.Since(bcR.lastHundred).Seconds())
		bcR.Logger.Info("Fast Sync Rate", "height", bcR.fsm.pool.height,
			"max_peer_height", bcR.fsm.pool.getMaxPeerHeight(), "blocks/s", bcR.lastRate)
		bcR.lastHundred = time.Now()
	}
	return nil
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (bcR *BlockchainReactor) poolRoutine() {

	for {
		select {
		case fromFSM := <-bcR.msgFromFSMCh:
			switch fromFSM.msgType {
			case errorMsg:
				peer := bcR.Switch.Peers().Get(fromFSM.error.peerID)
				if peer != nil {
					bcR.Switch.StopPeerForError(peer, fromFSM.error.err)
				}
			default:
			}
		}
	}
}

// TODO - send the error on msgFromFSMCh to process it above
func (bcR *BlockchainReactor) sendPeerError(err error, peerID p2p.ID) {
	peer := bcR.Switch.Peers().Get(peerID)
	if peer != nil {
		bcR.Switch.StopPeerForError(peer, err)
	}
}

func (bcR *BlockchainReactor) resetStateTimer(name string, timer *time.Timer, timeout time.Duration, f func()) {
	if timer == nil {
		timer = time.AfterFunc(timeout, f)
	} else {
		timer.Reset(timeout)
	}
}

// BroadcastStatusRequest broadcasts `BlockStore` height.
func (bcR *BlockchainReactor) sendStatusRequest() error {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{bcR.store.Height()})
	bcR.Switch.Broadcast(BlockchainChannel, msgBytes)
	return nil
}

// BlockRequest sends `BlockRequest` height.
func (bcR *BlockchainReactor) sendBlockRequest(peerID p2p.ID, height int64) error {
	peer := bcR.Switch.Peers().Get(peerID)

	if peer == nil {
		return errNilPeerForBlockRequest
	}

	msgBytes := cdc.MustMarshalBinaryBare(&bcBlockRequestMessage{height})
	queued := peer.TrySend(BlockchainChannel, msgBytes)
	if !queued {
		return errSendQueueFull
	}
	return nil
}

func (bcR *BlockchainReactor) switchToConsensus() {

	conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
	if ok {
		conR.SwitchToConsensus(bcR.state, bcR.blocksSynced)
	} else {
		// should only happen during testing
	}
}

//-----------------------------------------------------------------------------
// Messages

// BlockchainMessage is a generic message for this reactor.
type BlockchainMessage interface {
	ValidateBasic() error
}

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
	return m.Block.ValidateBasic()
}

func (m *bcBlockResponseMessage) String() string {
	return fmt.Sprintf("[bcBlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type bcStatusRequestMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcStatusRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *bcStatusRequestMessage) String() string {
	return fmt.Sprintf("[bcStatusRequestMessage %v]", m.Height)
}

//-------------------------------------

type bcStatusResponseMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *bcStatusResponseMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *bcStatusResponseMessage) String() string {
	return fmt.Sprintf("[bcStatusResponseMessage %v]", m.Height)
}
