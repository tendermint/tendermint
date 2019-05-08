package blockchain

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"

	"github.com/tendermint/tendermint/types"
)

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)
	trySyncIntervalMS = 10
	trySendIntervalMS = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10

	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
)

var (
	maxRequestsPerPeer    int32 = 40
	maxNumPendingRequests int32 = 500
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(sm.State, int)
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

	// Receive goroutine forwards messages to this channel to be processed in the context of the poolRoutine.
	messagesForFSMCh chan bcReactorMessage

	// Switch goroutine may send RemovePeer to the blockchain reactor. This is an error message that is relayed
	// to this channel to be processed in the context of the poolRoutine.
	errorsForFSMCh chan bcReactorMessage

	// This channel is used by the FSM and indirectly the block pool to report errors to the blockchain reactor and
	// the switch.
	eventsFromFSMCh chan bcFsmMessage
}

type BlockRequest struct {
	Height int64
	PeerID p2p.ID
}

// bcReactorMessage is used by the reactor to send messages to the FSM.
type bcReactorMessage struct {
	event bReactorEvent
	data  bReactorEventData
}

type bFsmEvent uint

const (
	// message type events
	peerErrorEv = iota + 1
	syncFinishedEv
)

type bFsmEventData struct {
	peerID p2p.ID
	err    error
}

// bcFsmMessage is used by the FSM to send messages to the reactor
type bcFsmMessage struct {
	event bFsmEvent
	data  bFsmEventData
}

// NewBlockchainReactor returns new reactor instance.
func NewBlockchainReactor(state sm.State, blockExec *sm.BlockExecutor, store *BlockStore,
	fastSync bool) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	const capacity = 1000
	eventsFromFSMCh := make(chan bcFsmMessage, capacity)
	messagesForFSMCh := make(chan bcReactorMessage, capacity)
	errorsForFSMCh := make(chan bcReactorMessage, capacity)

	bcR := &BlockchainReactor{
		initialState:     state,
		state:            state,
		blockExec:        blockExec,
		fastSync:         fastSync,
		store:            store,
		messagesForFSMCh: messagesForFSMCh,
		eventsFromFSMCh:  eventsFromFSMCh,
		errorsForFSMCh:   errorsForFSMCh,
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
		go bcR.poolRoutine()
	}
	return nil
}

// OnStop implements cmn.Service.
func (bcR *BlockchainReactor) OnStop() {
	_ = bcR.Stop()
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
	if !peer.TrySend(BlockchainChannel, msgBytes) {
		// doing nothing, will try later in `poolRoutine`
	}
	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.updatePeer()
}

// sendBlockToPeer loads a block and sends it to the requesting peer.
// If the block doesn't exist a bcNoBlockResponseMessage is sent.
// If all nodes are honest, no node should be requesting for a block that doesn't exist.
func (bcR *BlockchainReactor) sendBlockToPeer(msg *bcBlockRequestMessage,
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

func (bcR *BlockchainReactor) sendStatusResponseToPeer(msg *bcStatusRequestMessage, src p2p.Peer) (queued bool) {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{bcR.store.Height()})
	return src.TrySend(BlockchainChannel, msgBytes)
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	msgData := bcReactorMessage{
		event: peerRemoveEv,
		data: bReactorEventData{
			peerId: peer.ID(),
			err:    errSwitchRemovesPeer,
		},
	}
	bcR.errorsForFSMCh <- msgData
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
		if queued := bcR.sendBlockToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
			bcR.Logger.Error("Could not send block message to peer", "src", src, "height", msg.Height)
		}

	case *bcStatusRequestMessage:
		// Send peer our state.
		if queued := bcR.sendStatusResponseToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
			bcR.Logger.Error("Could not send status message to peer", "src", src)
		}

	case *bcBlockResponseMessage:
		msgData := bcReactorMessage{
			event: blockResponseEv,
			data: bReactorEventData{
				peerId: src.ID(),
				height: msg.Block.Height,
				block:  msg.Block,
				length: len(msgBytes),
			},
		}
		bcR.sendMessageToFSM(msgData)

	case *bcStatusResponseMessage:
		// Got a peer status. Unverified.
		msgData := bcReactorMessage{
			event: statusResponseEv,
			data: bReactorEventData{
				peerId: src.ID(),
				height: msg.Height,
				length: len(msgBytes),
			},
		}
		bcR.sendMessageToFSM(msgData)

	default:
		bcR.Logger.Error(fmt.Sprintf("unknown message type %v", reflect.TypeOf(msg)))
	}
}

func (bcR *BlockchainReactor) sendMessageToFSM(msg bcReactorMessage) {
	bcR.Logger.Debug("send message to FSM for processing", "msg", msg.String())
	bcR.messagesForFSMCh <- msg
}

// poolRoutine receives and handles messages from the Receive() routine and from the FSM
func (bcR *BlockchainReactor) poolRoutine() {

	bcR.fsm.start()

	processReceivedBlockTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	sendBlockRequestTicker := time.NewTicker(trySendIntervalMS * time.Millisecond)

	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)

	lastHundred := time.Now()
	lastRate := 0.0

	doProcessBlockCh := make(chan struct{}, 1)

ForLoop:
	for {
		select {

		case <-sendBlockRequestTicker.C:
			// Tell FSM to make more requests.
			// The maxNumPendingRequests may be changed based on low/ high watermark thresholds for
			// - the number of blocks received and waiting to be processed,
			// - the number of blockResponse messages waiting in messagesForFSMCh, etc.
			// Currently maxNumPendingRequests value is not changed.
			_ = bcR.fsm.handle(&bcReactorMessage{
				event: makeRequestsEv,
				data: bReactorEventData{
					maxNumRequests: maxNumPendingRequests}})

		case <-statusUpdateTicker.C:
			// Ask for status updates.
			go bcR.sendStatusRequest()

		case <-processReceivedBlockTicker.C: // chan time
			select {
			case doProcessBlockCh <- struct{}{}:
			default:
			}

		case <-doProcessBlockCh:
			err := bcR.processBlock()
			if err == errMissingBlocks {
				continue
			}
			// Notify FSM of block processing result.
			_ = bcR.fsm.handle(&bcReactorMessage{
				event: processedBlockEv,
				data: bReactorEventData{
					err: err,
				},
			})

			if err == errBlockVerificationFailure {
				continue
			}

			doProcessBlockCh <- struct{}{}

			bcR.blocksSynced++
			if bcR.blocksSynced%100 == 0 {
				lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
				bcR.Logger.Info("Fast Sync Rate", "height", bcR.fsm.pool.height,
					"max_peer_height", bcR.fsm.pool.getMaxPeerHeight(), "blocks/s", lastRate)
				lastHundred = time.Now()
			}

		case msg := <-bcR.messagesForFSMCh:
			_ = bcR.fsm.handle(&msg)

		case msg := <-bcR.errorsForFSMCh:
			_ = bcR.fsm.handle(&msg)

		case msg := <-bcR.eventsFromFSMCh:
			switch msg.event {
			case syncFinishedEv:
				break ForLoop
			case peerErrorEv:
				bcR.reportPeerErrorToSwitch(msg.data.err, msg.data.peerID)
				if msg.data.err == errNoPeerResponse {
					_ = bcR.fsm.handle(&bcReactorMessage{
						event: peerRemoveEv,
						data: bReactorEventData{
							peerId: msg.data.peerID,
							err:    msg.data.err,
						},
					})
				}
			default:
				bcR.Logger.Error("Event from FSM not supported", "type", msg.event)
			}

		case <-bcR.Quit():
			break ForLoop
		}
	}
}

func (bcR *BlockchainReactor) reportPeerErrorToSwitch(err error, peerID p2p.ID) {
	peer := bcR.Switch.Peers().Get(peerID)
	if peer != nil {
		bcR.Switch.StopPeerForError(peer, err)
	}
}

func (bcR *BlockchainReactor) processBlock() error {

	firstBP, secondBP, err := bcR.fsm.pool.getNextTwoBlocks()
	if err != nil {
		// We need both to sync the first block.
		return err
	}

	first := firstBP.block
	second := secondBP.block

	chainID := bcR.initialState.ChainID

	firstParts := first.MakePartSet(types.BlockPartSizeBytes)
	firstPartsHeader := firstParts.Header()
	firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}
	// Finally, verify the first block using the second's commit
	// NOTE: we can probably make this more efficient, but note that calling
	// first.Hash() doesn't verify the tx contents, so MakePartSet() is
	// currently necessary.
	err = bcR.state.Validators.VerifyCommit(
		chainID, firstID, first.Height, second.LastCommit)

	if err != nil {
		bcR.Logger.Error("error in validation", "err", err, first.Height, second.Height)
		return errBlockVerificationFailure
	}

	bcR.store.SaveBlock(first, firstParts, second.LastCommit)

	// Get the hash without persisting the state.
	bcR.state, err = bcR.blockExec.ApplyBlock(bcR.state, firstID, first)
	if err != nil {
		panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
	}
	return nil
}

// Implements bcRMessageInterface
// sendStatusRequest broadcasts `BlockStore` height.
func (bcR *BlockchainReactor) sendStatusRequest() {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{bcR.store.Height()})
	bcR.Switch.Broadcast(BlockchainChannel, msgBytes)
}

// Implements bcRMessageInterface
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

// Implements bcRMessageInterface
func (bcR *BlockchainReactor) switchToConsensus() {
	conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
	if ok {
		conR.SwitchToConsensus(bcR.state, bcR.blocksSynced)
		bcR.eventsFromFSMCh <- bcFsmMessage{event: syncFinishedEv}
	} else {
		// Should only happen during testing.
	}
}

// Implements bcRMessageInterface
// Called by FSM and pool:
// - pool calls when it detects slow peer or when peer times out
// - FSM calls when:
//    - adding a block (addBlock) fails
//    - reactor processing of a block reports failure and FSM sends back the peers of first and second blocks
func (bcR *BlockchainReactor) sendPeerError(err error, peerID p2p.ID) {
	msgData := bcFsmMessage{
		event: peerErrorEv,
		data: bFsmEventData{
			peerID: peerID,
			err:    err,
		},
	}
	bcR.eventsFromFSMCh <- msgData
}

// Implements bcRMessageInterface
func (bcR *BlockchainReactor) resetStateTimer(name string, timer **time.Timer, timeout time.Duration) {
	if timer == nil {
		panic("nil timer pointer parameter")
	}
	if *timer == nil {
		*timer = time.AfterFunc(timeout, func() {
			msg := bcReactorMessage{
				event: stateTimeoutEv,
				data: bReactorEventData{
					stateName: name,
				},
			}
			bcR.sendMessageToFSM(msg)
		})
	} else {
		(*timer).Reset(timeout)
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
