package blockchain_new

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

	// stop syncing when last block's time is
	// within this much of the system time.
	// stopSyncingDurationMinutes = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10
	// check if we should switch to consensus reactor
	switchToConsensusIntervalSeconds = 1

	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
)

var (
	maxRequestsPerPeer    int32 = 20
	maxNumPendingRequests int32 = 600
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
	messagesForFSMCh chan bReactorMessageData

	// Switch goroutine may send RemovePeer to the blockchain reactor. This is an error message that is relayed
	// to this channel to be processed in the context of the poolRoutine.
	errorsForFSMCh chan bReactorMessageData

	// This channel is used by the FSM and indirectly the block pool to report errors to the blockchain reactor and
	// the switch.
	errorsFromFSMCh chan peerError
}

type BlockRequest struct {
	Height int64
	PeerID p2p.ID
}

// bReactorMessageData structure is used by the reactor when sending messages to the FSM.
type bReactorMessageData struct {
	event bReactorEvent
	data  bReactorEventData
}

// NewBlockchainReactor returns new reactor instance.
func NewBlockchainReactor(state sm.State, blockExec *sm.BlockExecutor, store *BlockStore,
	fastSync bool) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	const capacity = 1000
	errorsFromFSMCh := make(chan peerError, capacity)
	messagesForFSMCh := make(chan bReactorMessageData, capacity)
	errorsForFSMCh := make(chan bReactorMessageData, capacity)

	bcR := &BlockchainReactor{
		initialState:     state,
		state:            state,
		blockExec:        blockExec,
		fastSync:         fastSync,
		store:            store,
		messagesForFSMCh: messagesForFSMCh,
		errorsFromFSMCh:  errorsFromFSMCh,
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
	if !peer.Send(BlockchainChannel, msgBytes) {
		// doing nothing, will try later in `poolRoutine`
	}
	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.SetPeerHeight
}

// respondToPeer loads a block and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
// According to the Tendermint spec, if all nodes are honest,
// no node should be requesting for a block that's non-existent.
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

func (bcR *BlockchainReactor) sendMessageToFSMAsync(msg bReactorMessageData) {
	bcR.Logger.Error("send message to FSM for processing", "msg", msg.String())
	bcR.messagesForFSMCh <- msg
}

func (bcR *BlockchainReactor) sendRemovePeerToFSM(peerID p2p.ID) {
	msgData := bReactorMessageData{
		event: peerRemoveEv,
		data: bReactorEventData{
			peerId: peerID,
		},
	}
	bcR.sendMessageToFSMAsync(msgData)
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	bcR.sendRemovePeerToFSM(peer.ID())
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
		msgData := bReactorMessageData{
			event: blockResponseEv,
			data: bReactorEventData{
				peerId: src.ID(),
				block:  msg.Block,
				length: len(msgBytes),
			},
		}
		bcR.sendMessageToFSMAsync(msgData)

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
		bcR.sendMessageToFSMAsync(msgData)

	default:
		bcR.Logger.Error(fmt.Sprintf("unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (bcR *BlockchainReactor) poolRoutine() {

	bcR.fsm.start()

	trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	trySendTicker := time.NewTicker(trySendIntervalMS * time.Millisecond)

	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
	switchToConsensusTicker := time.NewTicker(switchToConsensusIntervalSeconds * time.Second)

	lastHundred := time.Now()
	lastRate := 0.0

	doProcessCh := make(chan struct{}, 1)
	doSendCh := make(chan struct{}, 1)

ForLoop:
	for {
		select {

		case <-trySendTicker.C: // chan time
			select {
			case doSendCh <- struct{}{}:
			default:
			}
			//continue ForLoop

		case <-doSendCh:
			msgData := bReactorMessageData{
				event: makeRequestsEv,
				data: bReactorEventData{
					maxNumRequests: maxNumPendingRequests,
				},
			}
			_ = sendMessageToFSMSync(bcR.fsm, msgData)

		case err := <-bcR.errorsFromFSMCh:
			bcR.reportPeerErrorToSwitch(err.err, err.peerID)

		case <-statusUpdateTicker.C:
			// Ask for status updates.
			go bcR.sendStatusRequest()

		case <-switchToConsensusTicker.C:
			height, numPending, maxPeerHeight := bcR.fsm.pool.getStatus()
			outbound, inbound, _ := bcR.Switch.NumPeers()
			bcR.Logger.Debug("Consensus ticker", "numPending", numPending, "maxPeerHeight", maxPeerHeight,
				"outbound", outbound, "inbound", inbound)
			if bcR.fsm.isCaughtUp() {
				bcR.Logger.Info("Time to switch to consensus reactor!", "height", height)
				bcR.fsm.stop()
				bcR.switchToConsensus()
				break ForLoop
			}

		case <-trySyncTicker.C: // chan time
			select {
			case doProcessCh <- struct{}{}:
			default:
			}
			//continue FOR_LOOP

		case <-doProcessCh:
			err := bcR.processBlocksFromPoolRoutine()
			if err == errMissingBlocks {
				continue ForLoop
			}

			// Notify FSM of block processing result.
			msgData := bReactorMessageData{
				event: processedBlockEv,
				data: bReactorEventData{
					err: err,
				},
			}
			_ = sendMessageToFSMSync(bcR.fsm, msgData)

			if err == errBlockVerificationFailure {
				continue ForLoop
			}

			doProcessCh <- struct{}{}

			bcR.blocksSynced++
			if bcR.blocksSynced%100 == 0 {
				lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
				bcR.Logger.Info("Fast Sync Rate", "height", bcR.fsm.pool.height,
					"max_peer_height", bcR.fsm.pool.getMaxPeerHeight(), "blocks/s", lastRate)
				lastHundred = time.Now()
			}
			//continue ForLoop

		case msg := <-bcR.messagesForFSMCh:
			_ = sendMessageToFSMSync(bcR.fsm, msg)

		case msg := <-bcR.errorsForFSMCh:
			_ = sendMessageToFSMSync(bcR.fsm, msg)

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

// Called by FSM and pool:
// - pool calls when it detects slow peer or when peer times out
// - FSM calls when:
//    - processing a block (addBlock) fails
//    - BCR process of block reports failure to FSM, FSM sends back the peers of first and second
func (bcR *BlockchainReactor) sendPeerError(err error, peerID p2p.ID) {
	bcR.errorsFromFSMCh <- peerError{err, peerID}
}

func (bcR *BlockchainReactor) processBlocksFromPoolRoutine() error {
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

func (bcR *BlockchainReactor) resetStateTimer(name string, timer *time.Timer, timeout time.Duration, f func()) {
	if timer == nil {
		timer = time.AfterFunc(timeout, f)
	} else {
		timer.Reset(timeout)
	}
}

// BroadcastStatusRequest broadcasts `BlockStore` height.
func (bcR *BlockchainReactor) sendStatusRequest() {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{bcR.store.Height()})
	bcR.Switch.Broadcast(BlockchainChannel, msgBytes)
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
		// Should only happen during testing.
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
