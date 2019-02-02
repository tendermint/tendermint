package blockchain

import (
	"fmt"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/proxy"
	"reflect"
	"sync/atomic"
	"time"

	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

/*
	XXX: This file is copied from blockchain/reactor.go
*/

const (
	// StateChannel is a channel for state and status updates (`StateStore` height)
	StateChannel = byte(0x35)

	// check if we should retry sync
	retrySeconds = 30
	// how long should we start to request state (we need collect enough peers and their height but shouldn't make their app state pruned)
	startRequestStateSeconds = 10

	// if peers are lagged or beyond peerStateHeightThreshold, the app state might already pruned, we skip it (not syncing from it).
	peerStateHeightThreshold = 20

	// NOTE: keep up to date with bcStateResponseMessage
	// TODO: REVIEW before final merge
	bcStateResponseMessagePrefixSize   = 8
	bcStateResponseMessageFieldKeySize = 2
	maxStateMsgSize                    = types.MaxStateSizeBytes +
		bcStateResponseMessagePrefixSize +
		bcStateResponseMessageFieldKeySize
)

// BlockchainReactor handles long-term catchup syncing.
type StateReactor struct {
	p2p.BaseReactor

	// immutable
	initialState sm.State

	stateDB   dbm.DB
	app       proxy.AppConnState
	pool      *StatePool
	stateSync bool // positive for enable this reactor

	requestsCh <-chan StateRequest
	errorsCh   <-chan peerError
}

// NewBlockchainReactor returns new reactor instance.
func NewStateReactor(state sm.State, stateDB dbm.DB, app proxy.AppConnState, stateSync bool) *StateReactor {

	// TODO: revisit doesn't need
	//if state.LastBlockHeight != store.Height() {
	//	panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
	//		store.Height()))
	//}

	requestsCh := make(chan StateRequest, maxTotalRequesters)

	const capacity = 1000                      // must be bigger than peers count
	errorsCh := make(chan peerError, capacity) // so we don't block in #Receive#pool.AddBlock

	pool := NewStatePool(
		requestsCh,
		errorsCh,
	)

	bcSR := &StateReactor{
		initialState: state,
		stateDB:      stateDB,
		app:          app,
		pool:         pool,
		stateSync:    stateSync,
		requestsCh:   requestsCh,
		errorsCh:     errorsCh,
	}
	bcSR.BaseReactor = *p2p.NewBaseReactor("BlockchainStateReactor", bcSR)
	return bcSR
}

// SetLogger implements cmn.Service by setting the logger on reactor and pool.
func (bcSR *StateReactor) SetLogger(l log.Logger) {
	bcSR.BaseService.Logger = l
	bcSR.pool.Logger = l
}

// OnStart implements cmn.Service.
func (bcSR *StateReactor) OnStart() error {
	if bcSR.stateSync {
		err := bcSR.pool.Start()
		if err != nil {
			return err
		}
		go bcSR.poolRoutine()
	}
	return nil
}

// OnStop implements cmn.Service.
func (bcSR *StateReactor) OnStop() {
	bcSR.pool.Stop()
}

// GetChannels implements Reactor
func (_ *StateReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  StateChannel,
			Priority:            10,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxStateMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our state to peer.
func (bcSR *StateReactor) AddPeer(peer p2p.Peer) {
	// TODO: revisit whether to keep
	bcSR.Logger.Info("added a peer", "peer", peer.ID())
	//_, numKeys, _ := bcSR.app.LatestSnapshot()
	//msgBytes := cdc.MustMarshalBinaryBare(&bcStateStatusResponseMessage{sm.LoadState(bcSR.stateDB).LastBlockHeight, numKeys})
	//if !peer.Send(StateChannel, msgBytes) {
	//	// doing nothing, will try later in `poolRoutine`
	//}
	// peer is added to the pool once we receive the first
	// bcStateStatusResponseMessage from the peer and call pool.SetPeerHeight
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcSR *StateReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	bcSR.pool.RemovePeer(peer.ID())
}

// respondToPeer loads a state and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
// According to the Tendermint spec, if all nodes are honest,
// no node should be requesting for a state that's non-existent.
func (bcSR *StateReactor) respondToPeer(msg *bcStateRequestMessage,
	src p2p.Peer) (queued bool) {

	var state *sm.State
	if msg.StartIndex == 0 {
		if state = sm.LoadStateForHeight(bcSR.stateDB, msg.Height); state == nil {
			bcSR.Logger.Info("Peer asking for a state we don't have", "src", src, "height", msg.Height)

			msgBytes := cdc.MustMarshalBinaryBare(&bcNoStateResponseMessage{Height: msg.Height, StartIndex: msg.StartIndex, EndIndex: msg.EndIndex})
			return src.TrySend(StateChannel, msgBytes)
		}
	}

	chunk, err := bcSR.app.ReadSnapshotChunk(msg.Height, msg.StartIndex, msg.EndIndex)
	if err != nil {
		bcSR.Logger.Info("Peer asking for an application state we don't have", "src", src, "height", msg.Height, "err", err)

		msgBytes := cdc.MustMarshalBinaryBare(&bcNoStateResponseMessage{Height: msg.Height, StartIndex: msg.StartIndex, EndIndex: msg.EndIndex})
		return src.TrySend(StateChannel, msgBytes)
	}

	msgBytes := cdc.MustMarshalBinaryBare(&bcStateResponseMessage{State: state, StartIdxInc: msg.StartIndex, EndIdxExc: msg.EndIndex, Chunks: chunk})
	return src.TrySend(StateChannel, msgBytes)

}

// Receive implements Reactor by handling 4 types of messages (look below).
func (bcSR *StateReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeStateMsg(msgBytes)
	if err != nil {
		bcSR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		bcSR.Switch.StopPeerForError(src, err)
		return
	}

	bcSR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcStateRequestMessage:
		if queued := bcSR.respondToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
		}
	case *bcStateResponseMessage:
		// Got a block.
		//bcSR.pool.AddState(src.ID(), msg.State, len(msgBytes))
		//bcSR.pool.PopRequest()
		if msg.StartIdxInc == 0 {
			// first part should return state
			bcSR.pool.state = msg.State
			sm.SaveState(bcSR.stateDB, *msg.State)
		}
		bcSR.pool.AddStateChunk(src.ID(), msg)

		// received all chunks!
		if bcSR.pool.isComplete() {
			chunksToWrite := make([][]byte, 0, bcSR.pool.totalKeys)
			for startIdx := int64(0); startIdx < bcSR.pool.totalKeys; startIdx += bcSR.pool.step {
				if chunks, ok := bcSR.pool.chunks[startIdx]; ok {
					for _, chunk := range chunks {
						chunksToWrite = append(chunksToWrite, chunk)
					}
				} else {
					bcSR.Logger.Error("failed to locate state sync chunk", "startIdx", startIdx)
				}
			}
			err := bcSR.app.WriteRecoveryChunk(chunksToWrite)
			if err != nil {
				bcSR.Logger.Error("Failed to recover application state", "err", err)
			}

			bcSR.app.EndRecovery(msg.State.LastBlockHeight)

			bcSR.Logger.Info("Time to switch to blockchain reactor!", "height", msg.State.LastBlockHeight+1)
			bcSR.pool.Stop()

			bcR := bcSR.Switch.Reactor("BLOCKCHAIN").(*BlockchainReactor)
			bcR.SwitchToBlockchain(*msg.State)
		}

	case *bcStateStatusRequestMessage:
		// Send peer our state.
		height, numKeys, err := bcSR.app.LatestSnapshot()
		if err != nil {
			bcSR.Logger.Error("failed to load application state", "err", err)
		}
		state := sm.LoadState(bcSR.stateDB)
		if state.LastBlockHeight != height {
			bcSR.Logger.Error("application and state height is inconsistent")
		}
		msgBytes := cdc.MustMarshalBinaryBare(&bcStateStatusResponseMessage{state.LastBlockHeight, numKeys})
		queued := src.TrySend(StateChannel, msgBytes)
		if !queued {
			// sorry
		}
	case *bcStateStatusResponseMessage:
		// if pool is not initialized yet (this is first state status response), we init it
		if bcSR.pool.height == 0 {
			// TODO: make this a init method and make it thread safe
			bcSR.app.StartRecovery(msg.Height, msg.NumKeys)
			bcSR.pool.height = msg.Height
			bcSR.pool.numKeys = msg.NumKeys
			bcSR.pool.totalKeys = 0
			for _, numKey := range bcSR.pool.numKeys {
				bcSR.pool.totalKeys += numKey
			}
		}
		bcSR.pool.SetPeerHeight(src.ID(), msg.Height)
	default:
		bcSR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (bcSR *StateReactor) poolRoutine() {
	bcSR.BroadcastStateStatusRequest()
	retryTicker := time.NewTicker(retrySeconds * time.Second)
	startRequestStateTicker := time.NewTicker(startRequestStateSeconds * time.Second)

FOR_LOOP:
	for {
		select {
		case request := <-bcSR.requestsCh:
			peer := bcSR.Switch.Peers().Get(request.PeerID)
			if peer == nil {
				continue FOR_LOOP // Peer has since been disconnected.
			}
			msgBytes := cdc.MustMarshalBinaryBare(&bcStateRequestMessage{request.Height, request.StartIndex, request.EndIndex})
			queued := peer.TrySend(StateChannel, msgBytes)
			if !queued {
				// We couldn't make the request, send-queue full.
				// The pool handles timeouts, just let it go.
				continue FOR_LOOP
			}

		case err := <-bcSR.errorsCh:
			peer := bcSR.Switch.Peers().Get(err.peerID)
			if peer != nil {
				bcSR.Switch.StopPeerForError(peer, err)
			}

		case <-retryTicker.C:
			bcSR.pool.reset()
			bcSR.BroadcastStateStatusRequest()

		case <- startRequestStateTicker.C:
			if numPending := atomic.LoadInt32(&bcSR.pool.numPending); numPending == 0 {
				bcSR.pool.sendRequest()
			} else {
				bcSR.Logger.Info("still waiting for peer response for application state", "numPending", numPending)
			}

		case <-bcSR.Quit():
			break FOR_LOOP
		}
	}
}

// BroadcastStatusRequest broadcasts `StateStore` height.
func (bcSR *StateReactor) BroadcastStateStatusRequest() {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStateStatusRequestMessage{sm.LoadState(bcSR.stateDB).LastBlockHeight})
	bcSR.Switch.Broadcast(StateChannel, msgBytes)
}

//-----------------------------------------------------------------------------
// Messages

// BlockchainMessage is a generic message for this reactor.
type BlockchainStateMessage interface{}

func RegisterBlockchainStateMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*BlockchainStateMessage)(nil), nil)
	cdc.RegisterConcrete(&bcStateRequestMessage{}, "tendermint/blockchain/StateRequest", nil)
	cdc.RegisterConcrete(&bcStateResponseMessage{}, "tendermint/blockchain/StateResponse", nil)
	cdc.RegisterConcrete(&bcNoStateResponseMessage{}, "tendermint/blockchain/NoStateResponse", nil)
	cdc.RegisterConcrete(&bcStateStatusResponseMessage{}, "tendermint/blockchain/StateStatusResponse", nil)
	cdc.RegisterConcrete(&bcStateStatusRequestMessage{}, "tendermint/blockchain/StateStatusRequest", nil)
}

func decodeStateMsg(bz []byte) (msg BlockchainStateMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("State msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------

type bcStateRequestMessage struct {
	Height int64
	StartIndex int64
	EndIndex int64
}

func (m *bcStateRequestMessage) String() string {
	return fmt.Sprintf("[bcStateRequestMessage %v]", m.Height)
}

type bcNoStateResponseMessage struct {
	Height int64
	StartIndex int64
	EndIndex int64
}

func (brm *bcNoStateResponseMessage) String() string {
	return fmt.Sprintf("[bcNoStateResponseMessage %d]", brm.Height)
}

//-------------------------------------

type bcStateResponseMessage struct {
	State       *sm.State
	StartIdxInc int64
	EndIdxExc   int64
	Chunks      [][]byte // len(Chunks) == (EndIndex - StartIndex)
}

func (m *bcStateResponseMessage) String() string {
	return fmt.Sprintf("[bcStateResponseMessage %v]", m.State.LastBlockHeight)
}

//-------------------------------------

type bcStateStatusRequestMessage struct {
	Height int64
}

func (m *bcStateStatusRequestMessage) String() string {
	return fmt.Sprintf("[bcStateStatusRequestMessage %v]", m.Height)
}

//-------------------------------------

type bcStateStatusResponseMessage struct {
	Height int64
	NumKeys   []int64
}

func (m *bcStateStatusResponseMessage) String() string {
	return fmt.Sprintf("[bcStateStatusResponseMessage %v]", m.Height)
}
