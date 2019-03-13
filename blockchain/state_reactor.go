package blockchain

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/proxy"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"time"

	cfg "github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// StateChannel is a channel for state and status updates (`StateStore` height)
	StateChannel = byte(0x35)

	// check if we should retry broadcast state status msg
	retrySeconds = 5
	// check if we are timeout and retry request again, 20 minutes here
	timeoutSeconds = 1200
	// how long should we start to request state (we need collect enough peers and their height but shouldn't make their app state pruned)
	startRequestStateSeconds = 10

	// if peers are lagged or beyond peerStateHeightThreshold, the app state might already pruned, we skip it (not syncing from it).
	peerStateHeightThreshold = 20

	// NOTE: keep up to date with bcStateResponseMessage
	// TODO: REVIEW before final merge
	bcStateResponseMessagePrefixSize   = 8
	bcStateResponseMessageFieldKeySize = 2
	keysPerRequest                     = 5000 // should be around 500K
	maxStateMsgSize                    = types.MaxStateSizeBytes +
		bcStateResponseMessagePrefixSize +
		bcStateResponseMessageFieldKeySize

	// Currently we only allow user state sync at first time when they connecting to network
	// otherwise there will be "holes" on their blockstore which might cause other new peers retry block sync
	// This will be improved afterwards (release with mainnet)
	lockFileName = "STATESYNC.LOCK"
)

// BlockchainReactor handles long-term catchup syncing.
type StateReactor struct {
	p2p.BaseReactor

	stateDB   dbm.DB
	app       proxy.AppConnState
	config    *cfg.Config
	pool      *StatePool
	stateSync bool // positive for enable this reactor

	requestsCh <-chan StateRequest
	errorsCh   <-chan peerError
	quitCh chan struct{}
}

// NewStateReactor returns new reactor instance.
func NewStateReactor(stateDB dbm.DB, app proxy.AppConnState, config *cfg.Config) *StateReactor {
	requestsCh := make(chan StateRequest, maxTotalRequesters)

	const capacity = 1000                      // must be bigger than peers count
	errorsCh := make(chan peerError, capacity) // so we don't block in #Receive#pool.AddBlock

	pool := NewStatePool(
		requestsCh,
		errorsCh,
	)

	bcSR := &StateReactor{
		stateDB:      stateDB,
		app:          app,
		config: 	  config,
		pool:         pool,
		stateSync:    config.StateSync,
		requestsCh:   requestsCh,
		errorsCh:     errorsCh,
		quitCh: 	  make(chan struct{}),
	}
	bcSR.BaseReactor = *p2p.NewBaseReactor("BlockchainStateReactor", bcSR)

	lockFilePath := filepath.Join(config.DBDir(), lockFileName)
	if _, err := os.Stat(lockFilePath); !os.IsNotExist(err) {
		// if we already state sycned, we modify the config so that fast sync and consensus reactor is not impacted
		// setting here is to make sure there is an error log for user when state reactor started (logger is not initialized here)
		config.StateSync = false
	}

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
		// we only allow state sync on node fist start, see comment of `lockFileName`
		lockFilePath := filepath.Join(bcSR.config.DBDir(), lockFileName)
		if _, err := os.Stat(lockFilePath); os.IsNotExist(err) {
			if _, err := os.Create(lockFilePath); err != nil {
				bcSR.Logger.Error("failed to create state sync lock file", "err", err)
				return err
			}
			if err := bcSR.pool.Start(); err != nil {
				return err
			}
			go bcSR.poolRoutine()
		} else {
			bcSR.Logger.Error("This node might has state synced, will fast sync! Please consider unsafe_reset_all if it is lag behind too much", "err", err)
			return err
		}
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
	// deliberately do nothing when add peer, because LatestSnapshot operation is too expensive
	// only response when receive bcStateStatusRequestMessage
	bcSR.Logger.Info("added a peer", "peer", peer.ID())
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
			if msg, err := bcSR.compress(msgBytes); err == nil {
				return src.TrySend(StateChannel, msg)
			} else {
				bcSR.Logger.Error("failed to compress bcNoStateResponseMessage", "err", err)
			}
		}
	}

	chunk, err := bcSR.app.ReadSnapshotChunk(msg.Height, msg.StartIndex, msg.EndIndex)
	if err != nil {
		bcSR.Logger.Info("Peer asking for an application state we don't have", "src", src, "height", msg.Height, "err", err)

		msgBytes := cdc.MustMarshalBinaryBare(&bcNoStateResponseMessage{Height: msg.Height, StartIndex: msg.StartIndex, EndIndex: msg.EndIndex})
		if msg, err := bcSR.compress(msgBytes); err == nil {
			return src.TrySend(StateChannel, msg)
		} else {
			bcSR.Logger.Error("failed to compressed bcNoStateResponseMessage", "err", err)
		}
	}

	msgBytes := cdc.MustMarshalBinaryBare(&bcStateResponseMessage{State: state, StartIdxInc: msg.StartIndex, EndIdxExc: msg.EndIndex, Chunks: chunk})
	if msg, err := bcSR.compress(msgBytes); err == nil {
		return src.TrySend(StateChannel, msg)
	} else {
		bcSR.Logger.Error("failed to compress bcStateResponseMessage", "err", err)
		return false
	}
}

// Receive implements Reactor by handling 4 types of messages (see below).
func (bcSR *StateReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := bcSR.decodeStateMsg(msgBytes)
	if err != nil {
		bcSR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		bcSR.Switch.StopPeerForError(src, err)
		return
	}

	//bcSR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcStateRequestMessage:
		if queued := bcSR.respondToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
		}
	case *bcStateResponseMessage:
		if msg.StartIdxInc == 0 {
			// first part should return state
			bcSR.pool.state = msg.State
			sm.SaveState(bcSR.stateDB, *msg.State)
		}
		bcSR.pool.AddStateChunk(src.ID(), msg)

		// received all chunks!
		if bcSR.pool.isComplete() {
			bcSR.quitCh <- struct{}{}

			chunksToWrite := make([][]byte, 0, bcSR.pool.totalKeys)
			for startIdx := int64(0); startIdx < bcSR.pool.totalKeys; startIdx += keysPerRequest {
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

			bcSR.app.EndRecovery(bcSR.pool.state.LastBlockHeight)

			bcSR.Logger.Info("Time to switch to blockchain reactor!", "height", bcSR.pool.state.LastBlockHeight+1)

			bcR := bcSR.Switch.Reactor("BLOCKCHAIN").(*BlockchainReactor)
			bcR.SwitchToBlockchain(bcSR.pool.state)
		}

	case *bcStateStatusRequestMessage:
		// Send peer our state.
		height, numKeys, err := bcSR.app.LatestSnapshot()
		if err != nil {
			bcSR.Logger.Error("failed to load application state", "err", err)
		}
		msgBytes := cdc.MustMarshalBinaryBare(&bcStateStatusResponseMessage{height, numKeys})
		if msg, err := bcSR.compress(msgBytes); err == nil {
			queued := src.TrySend(StateChannel, msg)
			if !queued {
				bcSR.Logger.Error("failed to queue bcStateStatusResponseMessage")
			}
		} else {
			bcSR.Logger.Error("failed to compress bcStateStatusResponseMessage", "err", err)
		}

	case *bcStateStatusResponseMessage:
		if msg.Height == 0 {
			bcR := bcSR.Switch.Reactor("BLOCKCHAIN").(*BlockchainReactor)
			bcSR.quitCh <- struct{}{}
			bcR.SwitchToBlockchain(nil)
		} else {
			// if pool is not initialized yet (this is first state status response), we init it
			if bcSR.pool.height == 0 {
				bcSR.app.StartRecovery(msg.Height, msg.NumKeys)
				bcSR.pool.init(msg)
			}
			bcSR.pool.SetPeerHeight(src.ID(), msg.Height)
		}
	default:
		bcSR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (bcSR *StateReactor) poolRoutine() {
	bcSR.BroadcastStateStatusRequest()
	retryTicker := time.NewTicker(retrySeconds * time.Second)
	timeoutTicker := time.NewTicker(timeoutSeconds * time.Second)
	startRequestStateTicker := time.NewTicker(startRequestStateSeconds * time.Second)
	defer retryTicker.Stop()
	defer timeoutTicker.Stop()
	defer startRequestStateTicker.Stop()
FOR_LOOP:
	for {
		select {
		case request := <-bcSR.requestsCh:
			peer := bcSR.Switch.Peers().Get(request.PeerID)
			if peer == nil {
				continue FOR_LOOP // Peer has since been disconnected.
			}
			msgBytes := cdc.MustMarshalBinaryBare(&bcStateRequestMessage{request.Height, request.StartIndex, request.EndIndex})
			bcSR.Logger.Info("try to request state", "peer", peer.ID(), "startIdx", request.StartIndex, "endIdx", request.EndIndex)
			if msg, err := bcSR.compress(msgBytes); err == nil {
				queued := peer.TrySend(StateChannel, msg)
				if !queued {
					// We couldn't make the request, send-queue full.
					// The pool handles timeouts, just let it go.
					continue FOR_LOOP
				}
			} else {
				bcSR.Logger.Error("failed to compress bcStateRequestMessage", "err", err)
			}
		case err := <-bcSR.errorsCh:
			peer := bcSR.Switch.Peers().Get(err.peerID)
			if peer != nil {
				bcSR.Switch.StopPeerForError(peer, err)
			}

		case <-retryTicker.C:
			bcSR.BroadcastStateStatusRequest()

		case <- timeoutTicker.C:
			bcSR.pool.reset()
			bcSR.BroadcastStateStatusRequest()

		case <- startRequestStateTicker.C:
			if numPending := atomic.LoadInt32(&bcSR.pool.numPending); numPending == 0 {
				bcSR.pool.sendRequest()
			} else {
				bcSR.Logger.Info("still waiting for peer response for application state", "numPending", numPending)
			}

		// we don't want sync state anymore, but we still need keep reactor running to serve state request
		case <- bcSR.quitCh:
			break FOR_LOOP

		case <-bcSR.Quit():
			break FOR_LOOP
		}
	}
}

// BroadcastStatusRequest broadcasts `StateStore` height.
func (bcSR *StateReactor) BroadcastStateStatusRequest() {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStateStatusRequestMessage{sm.LoadState(bcSR.stateDB).LastBlockHeight})
	if msg, err := bcSR.compress(msgBytes); err == nil {
		bcSR.Switch.Broadcast(StateChannel, msg)
	} else {
		bcSR.Logger.Error("failed to compress bcStateStatusRequestMessage", "err", err)
	}
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

func (bcSR *StateReactor) decodeStateMsg(bz []byte) (msg BlockchainStateMessage, err error) {
	if len(bz) > maxStateMsgSize {
		return msg, fmt.Errorf("State msg exceeds max size (%d > %d)", len(bz), maxStateMsgSize)
	}

	if decompressedBz, decompressErr := bcSR.decompress(bz); decompressErr == nil {
		err = cdc.UnmarshalBinaryBare(decompressedBz, &msg)
		return msg, err
	} else {
		return nil, decompressErr
	}

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
	NumKeys   []int64	// numKeys we are expected, in app defined sub store order
}

func (m *bcStateStatusResponseMessage) String() string {
	return fmt.Sprintf("[bcStateStatusResponseMessage %v]", m.Height)
}

func (bcSR *StateReactor) compress(toBeCompressed []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(toBeCompressed)
	if err != nil {
		bcSR.Logger.Error("failed to compress", "err", err)
		return nil, err
	}

	// deliberately not put in defer to save a flush call
	if err := zw.Close(); err != nil {
		bcSR.Logger.Error("failed to close compress buffer", "err", err)
		return nil, err
	}

	ret := buf.Bytes()
	bcSR.Logger.Debug("compressed state reactor message", "from", len(toBeCompressed), "to", len(ret))
	return ret, nil
}

func (bcSR *StateReactor) decompress(toBeDecompressed []byte) ([]byte, error) {
	buf := bytes.NewBuffer(toBeDecompressed)
	var resBuf bytes.Buffer
	zr, err := gzip.NewReader(buf)
	if err != nil {
		bcSR.Logger.Error("failed to create decompress reader", "err", err)
		return nil, err
	}

	if _, err := io.Copy(&resBuf, zr); err != nil {
		bcSR.Logger.Error("failed to decompress", "err", err)
		return nil, err
	}

	// deliberately not put in defer to save a flush call
	if err := zr.Close(); err != nil {
		bcSR.Logger.Error("failed to close decompressor", "err", err)
		return nil, err
	}
	return resBuf.Bytes(), nil
}
