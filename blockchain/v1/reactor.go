package v1

import (
	"fmt"
	"reflect"
	"time"

	"github.com/tendermint/tendermint/behaviour"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)
	trySyncIntervalMS = 10
	trySendIntervalMS = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10
)

var (
	// Maximum number of requests that can be pending per peer, i.e. for which requests have been sent but blocks
	// have not been received.
	maxRequestsPerPeer = 20
	// Maximum number of block requests for the reactor, pending or for which blocks have been received.
	maxNumRequests = 64
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(state sm.State, skipWAL bool)
}

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	p2p.BaseReactor

	initialState sm.State // immutable
	state        sm.State

	blockExec *sm.BlockExecutor
	store     *store.BlockStore

	fastSync    bool
	stateSynced bool

	fsm          *BcReactorFSM
	blocksSynced uint64

	// Receive goroutine forwards messages to this channel to be processed in the context of the poolRoutine.
	messagesForFSMCh chan bcReactorMessage

	// Switch goroutine may send RemovePeer to the blockchain reactor. This is an error message that is relayed
	// to this channel to be processed in the context of the poolRoutine.
	errorsForFSMCh chan bcReactorMessage

	// This channel is used by the FSM and indirectly the block pool to report errors to the blockchain reactor and
	// the switch.
	eventsFromFSMCh chan bcFsmMessage

	swReporter *behaviour.SwitchReporter
}

// NewBlockchainReactor returns new reactor instance.
func NewBlockchainReactor(state sm.State, blockExec *sm.BlockExecutor, store *store.BlockStore,
	fastSync bool) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	const capacity = 1000
	eventsFromFSMCh := make(chan bcFsmMessage, capacity)
	messagesForFSMCh := make(chan bcReactorMessage, capacity)
	errorsForFSMCh := make(chan bcReactorMessage, capacity)

	startHeight := store.Height() + 1
	if startHeight == 1 {
		startHeight = state.InitialHeight
	}
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
	fsm := NewFSM(startHeight, bcR)
	bcR.fsm = fsm
	bcR.BaseReactor = *p2p.NewBaseReactor("BlockchainReactor", bcR)
	// bcR.swReporter = behaviour.NewSwitchReporter(bcR.BaseReactor.Switch)

	return bcR
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

// SetLogger implements service.Service by setting the logger on reactor and pool.
func (bcR *BlockchainReactor) SetLogger(l log.Logger) {
	bcR.BaseService.Logger = l
	bcR.fsm.SetLogger(l)
}

// OnStart implements service.Service.
func (bcR *BlockchainReactor) OnStart() error {
	bcR.swReporter = behaviour.NewSwitchReporter(bcR.BaseReactor.Switch)
	if bcR.fastSync {
		go bcR.poolRoutine()
	}
	return nil
}

// OnStop implements service.Service.
func (bcR *BlockchainReactor) OnStop() {
	_ = bcR.Stop()
}

// SwitchToFastSync is called by the state sync reactor when switching to fast sync.
func (bcR *BlockchainReactor) SwitchToFastSync(state sm.State) error {
	bcR.fastSync = true
	bcR.initialState = state
	bcR.state = state
	bcR.stateSynced = true

	bcR.fsm = NewFSM(state.LastBlockHeight+1, bcR)
	bcR.fsm.SetLogger(bcR.Logger)
	go bcR.poolRoutine()
	return nil
}

// GetChannels implements Reactor
func (bcR *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            10,
			SendQueueCapacity:   2000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: bc.MaxMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our state to peer.
func (bcR *BlockchainReactor) AddPeer(peer p2p.Peer) {
	msgBytes, err := bc.EncodeMsg(&bcproto.StatusResponse{
		Base:   bcR.store.Base(),
		Height: bcR.store.Height(),
	})
	if err != nil {
		bcR.Logger.Error("could not convert msg to protobuf", "err", err)
		return
	}
	peer.Send(BlockchainChannel, msgBytes)
	// it's OK if send fails. will try later in poolRoutine

	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.updatePeer()
}

// sendBlockToPeer loads a block and sends it to the requesting peer.
// If the block doesn't exist a bcNoBlockResponseMessage is sent.
// If all nodes are honest, no node should be requesting for a block that doesn't exist.
func (bcR *BlockchainReactor) sendBlockToPeer(msg *bcproto.BlockRequest,
	src p2p.Peer) (queued bool) {

	block := bcR.store.LoadBlock(msg.Height)
	if block != nil {
		pbbi, err := block.ToProto()
		if err != nil {
			bcR.Logger.Error("Could not send block message to peer", "err", err)
			return false
		}
		msgBytes, err := bc.EncodeMsg(&bcproto.BlockResponse{Block: pbbi})
		if err != nil {
			bcR.Logger.Error("unable to marshal msg", "err", err)
			return false
		}
		return src.TrySend(BlockchainChannel, msgBytes)
	}

	bcR.Logger.Info("peer asking for a block we don't have", "src", src, "height", msg.Height)

	msgBytes, err := bc.EncodeMsg(&bcproto.NoBlockResponse{Height: msg.Height})
	if err != nil {
		bcR.Logger.Error("unable to marshal msg", "err", err)
		return false
	}
	return src.TrySend(BlockchainChannel, msgBytes)
}

func (bcR *BlockchainReactor) sendStatusResponseToPeer(msg *bcproto.StatusRequest, src p2p.Peer) (queued bool) {
	msgBytes, err := bc.EncodeMsg(&bcproto.StatusResponse{
		Base:   bcR.store.Base(),
		Height: bcR.store.Height(),
	})
	if err != nil {
		bcR.Logger.Error("unable to marshal msg", "err", err)
		return false
	}

	return src.TrySend(BlockchainChannel, msgBytes)
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	msgData := bcReactorMessage{
		event: peerRemoveEv,
		data: bReactorEventData{
			peerID: peer.ID(),
			err:    errSwitchRemovesPeer,
		},
	}
	bcR.errorsForFSMCh <- msgData
}

// Receive implements Reactor by handling 4 types of messages (look below).
func (bcR *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := bc.DecodeMsg(msgBytes)
	if err != nil {
		bcR.Logger.Error("error decoding message", "src", src, "chId", chID, "err", err)
		_ = bcR.swReporter.Report(behaviour.BadMessage(src.ID(), err.Error()))
		return
	}

	if err = bc.ValidateMsg(msg); err != nil {
		bcR.Logger.Error("peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		_ = bcR.swReporter.Report(behaviour.BadMessage(src.ID(), err.Error()))
		return
	}

	bcR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcproto.BlockRequest:
		if queued := bcR.sendBlockToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
			bcR.Logger.Error("Could not send block message to peer", "src", src, "height", msg.Height)
		}

	case *bcproto.StatusRequest:
		// Send peer our state.
		if queued := bcR.sendStatusResponseToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
			bcR.Logger.Error("Could not send status message to peer", "src", src)
		}

	case *bcproto.BlockResponse:
		bi, err := types.BlockFromProto(msg.Block)
		if err != nil {
			bcR.Logger.Error("error transition block from protobuf", "err", err)
			return
		}
		msgForFSM := bcReactorMessage{
			event: blockResponseEv,
			data: bReactorEventData{
				peerID: src.ID(),
				height: bi.Height,
				block:  bi,
				length: len(msgBytes),
			},
		}
		bcR.Logger.Info("Received", "src", src, "height", bi.Height)
		bcR.messagesForFSMCh <- msgForFSM
	case *bcproto.NoBlockResponse:
		msgForFSM := bcReactorMessage{
			event: noBlockResponseEv,
			data: bReactorEventData{
				peerID: src.ID(),
				height: msg.Height,
			},
		}
		bcR.Logger.Debug("Peer does not have requested block", "peer", src, "height", msg.Height)
		bcR.messagesForFSMCh <- msgForFSM

	case *bcproto.StatusResponse:
		// Got a peer status. Unverified.
		msgForFSM := bcReactorMessage{
			event: statusResponseEv,
			data: bReactorEventData{
				peerID: src.ID(),
				height: msg.Height,
				length: len(msgBytes),
			},
		}
		bcR.messagesForFSMCh <- msgForFSM

	default:
		bcR.Logger.Error(fmt.Sprintf("unknown message type %v", reflect.TypeOf(msg)))
	}
}

// processBlocksRoutine processes blocks until signlaed to stop over the stopProcessing channel
func (bcR *BlockchainReactor) processBlocksRoutine(stopProcessing chan struct{}) {

	processReceivedBlockTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	doProcessBlockCh := make(chan struct{}, 1)

	lastHundred := time.Now()
	lastRate := 0.0

ForLoop:
	for {
		select {
		case <-stopProcessing:
			bcR.Logger.Info("finishing block execution")
			break ForLoop
		case <-processReceivedBlockTicker.C: // try to execute blocks
			select {
			case doProcessBlockCh <- struct{}{}:
			default:
			}
		case <-doProcessBlockCh:
			for {
				err := bcR.processBlock()
				if err == errMissingBlock {
					break
				}
				// Notify FSM of block processing result.
				msgForFSM := bcReactorMessage{
					event: processedBlockEv,
					data: bReactorEventData{
						err: err,
					},
				}
				_ = bcR.fsm.Handle(&msgForFSM)

				if err != nil {
					break
				}

				bcR.blocksSynced++
				if bcR.blocksSynced%100 == 0 {
					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					height, maxPeerHeight := bcR.fsm.Status()
					bcR.Logger.Info("Fast Sync Rate", "height", height,
						"max_peer_height", maxPeerHeight, "blocks/s", lastRate)
					lastHundred = time.Now()
				}
			}
		}
	}
}

// poolRoutine receives and handles messages from the Receive() routine and from the FSM.
func (bcR *BlockchainReactor) poolRoutine() {

	bcR.fsm.Start()

	sendBlockRequestTicker := time.NewTicker(trySendIntervalMS * time.Millisecond)
	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)

	stopProcessing := make(chan struct{}, 1)
	go bcR.processBlocksRoutine(stopProcessing)

ForLoop:
	for {
		select {

		case <-sendBlockRequestTicker.C:
			if !bcR.fsm.NeedsBlocks() {
				continue
			}
			_ = bcR.fsm.Handle(&bcReactorMessage{
				event: makeRequestsEv,
				data: bReactorEventData{
					maxNumRequests: maxNumRequests}})

		case <-statusUpdateTicker.C:
			// Ask for status updates.
			go bcR.sendStatusRequest()

		case msg := <-bcR.messagesForFSMCh:
			// Sent from the Receive() routine when status (statusResponseEv) and
			// block (blockResponseEv) response events are received
			_ = bcR.fsm.Handle(&msg)

		case msg := <-bcR.errorsForFSMCh:
			// Sent from the switch.RemovePeer() routine (RemovePeerEv) and
			// FSM state timer expiry routine (stateTimeoutEv).
			_ = bcR.fsm.Handle(&msg)

		case msg := <-bcR.eventsFromFSMCh:
			switch msg.event {
			case syncFinishedEv:
				stopProcessing <- struct{}{}
				// Sent from the FSM when it enters finished state.
				break ForLoop
			case peerErrorEv:
				// Sent from the FSM when it detects peer error
				bcR.reportPeerErrorToSwitch(msg.data.err, msg.data.peerID)
				if msg.data.err == errNoPeerResponse {
					// Sent from the peer timeout handler routine
					_ = bcR.fsm.Handle(&bcReactorMessage{
						event: peerRemoveEv,
						data: bReactorEventData{
							peerID: msg.data.peerID,
							err:    msg.data.err,
						},
					})
				}
				// else {
				// For slow peers, or errors due to blocks received from wrong peer
				// the FSM had already removed the peers
				// }
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
		_ = bcR.swReporter.Report(behaviour.BadMessage(peerID, err.Error()))
	}
}

func (bcR *BlockchainReactor) processBlock() error {

	first, second, err := bcR.fsm.FirstTwoBlocks()
	if err != nil {
		// We need both to sync the first block.
		return err
	}

	chainID := bcR.initialState.ChainID

	firstParts := first.MakePartSet(types.BlockPartSizeBytes)
	firstPartSetHeader := firstParts.Header()
	firstID := types.BlockID{Hash: first.Hash(), PartSetHeader: firstPartSetHeader}
	firstStateID := types.StateID{LastAppHash: first.Header.AppHash}
	// Finally, verify the first block using the second's commit
	// NOTE: we can probably make this more efficient, but note that calling
	// first.Hash() doesn't verify the tx contents, so MakePartSet() is
	// currently necessary.
	err = bcR.state.Validators.VerifyCommitLight(chainID, firstID, firstStateID, first.Height, second.LastCommit)
	if err != nil {
		bcR.Logger.Error("error during commit verification", "err", err,
			"first", first.Height, "second", second.Height)
		return errBlockVerificationFailure
	}

	bcR.store.SaveBlock(first, firstParts, second.LastCommit)

	bcR.state, _, err = bcR.blockExec.ApplyBlock(bcR.state, firstID, first)
	if err != nil {
		panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
	}

	return nil
}

// Implements bcRNotifier
// sendStatusRequest broadcasts `BlockStore` height.
func (bcR *BlockchainReactor) sendStatusRequest() {
	msgBytes, err := bc.EncodeMsg(&bcproto.StatusRequest{})
	if err != nil {
		panic(err)
	}
	bcR.Switch.Broadcast(BlockchainChannel, msgBytes)
}

// Implements bcRNotifier
// BlockRequest sends `BlockRequest` height.
func (bcR *BlockchainReactor) sendBlockRequest(peerID p2p.ID, height int64) error {
	peer := bcR.Switch.Peers().Get(peerID)
	if peer == nil {
		return errNilPeerForBlockRequest
	}

	msgBytes, err := bc.EncodeMsg(&bcproto.BlockRequest{Height: height})
	if err != nil {
		return err
	}
	queued := peer.TrySend(BlockchainChannel, msgBytes)
	if !queued {
		return errSendQueueFull
	}
	return nil
}

// Implements bcRNotifier
func (bcR *BlockchainReactor) switchToConsensus() {
	conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
	if ok {
		conR.SwitchToConsensus(bcR.state, bcR.blocksSynced > 0 || bcR.stateSynced)
		bcR.eventsFromFSMCh <- bcFsmMessage{event: syncFinishedEv}
	}
	// else {
	// Should only happen during testing.
	// }
}

// Implements bcRNotifier
// Called by FSM and pool:
// - pool calls when it detects slow peer or when peer times out
// - FSM calls when:
//    - adding a block (addBlock) fails
//    - reactor processing of a block reports failure and FSM sends back the peers of first and second blocks
func (bcR *BlockchainReactor) sendPeerError(err error, peerID p2p.ID) {
	bcR.Logger.Info("sendPeerError:", "peer", peerID, "error", err)
	msgData := bcFsmMessage{
		event: peerErrorEv,
		data: bFsmEventData{
			peerID: peerID,
			err:    err,
		},
	}
	bcR.eventsFromFSMCh <- msgData
}

// Implements bcRNotifier
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
			bcR.errorsForFSMCh <- msg
		})
	} else {
		(*timer).Reset(timeout)
	}
}
