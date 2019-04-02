package snapshot

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/golang/snappy"

	"github.com/tendermint/go-amino"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

const (
	// StateSyncChannel is a channel for state and status updates (`StateStore` height)
	StateSyncChannel = byte(0x35)

	// ====== move to config ======
	stateSyncPriority = 10			// channel priority of state sync
	tryManifestFinalizeSeconds = 10	// how often to check whether we collect enough manifests
	maxTriedManifestFinalist = 3	// max round of try finalize manifest
	leastPeersToSync = 3	// how many peers with same manifest hash before we can start state sync

	// NOTE: keep up to date with bcChunkResponseMessage
	// TODO: REVIEW before final merge
	bcStateResponseMessagePrefixSize   = 8
	bcStateResponseMessageFieldKeySize = 2
	maxStateMsgSize                    = types.MaxStateSizeBytes +
		bcStateResponseMessagePrefixSize +
		bcStateResponseMessageFieldKeySize
	maxInFlightRequesters        	   = 600

	maxInflightRequestPerPeer = 20

	// Currently we only allow user state sync at first time when they connecting to network
	// otherwise there will be "holes" on their blockstore which might cause other new peers retry block sync
	stateSyncLockFileName = "STATESYNC.LOCK"
)

type peerError struct {
	err    error
	peerID p2p.ID
}

// BlockchainReactor handles long-term catchup syncing.
type StateReactor struct {
	p2p.BaseReactor

	config    *cfg.Config
	pool      *StatePool
	stateSync bool // positive for enable this reactor

	requestsCh <-chan SnapshotRequest
	errorsCh   <-chan peerError
}

// NewStateReactor returns new reactor instance.
func NewStateReactor(stateDB dbm.DB, app proxy.AppConnState, config *cfg.Config) *StateReactor {
	requestsCh := make(chan SnapshotRequest, maxInFlightRequesters)

	const capacity = 1000                      // must be bigger than peers count
	errorsCh := make(chan peerError, capacity) // so we don't block in #Receive#pool.AddBlock

	pool := NewStatePool(
		app,
		requestsCh,
		errorsCh,
		stateDB,
	)

	lockFilePath := filepath.Join(config.DBDir(), stateSyncLockFileName)
	if _, err := os.Stat(lockFilePath); !os.IsNotExist(err) {
		// if we already state synced, we modify the config so that fast sync and consensus reactor is not impacted
		// setting here is to make sure there is an error log for user when state reactor started (logger is not initialized here)
		config.StateSync = false
	}

	bcSR := &StateReactor{
		config: 	  config,
		pool:         pool,
		stateSync:    config.StateSync,
		requestsCh:   requestsCh,
		errorsCh:     errorsCh,
	}
	bcSR.BaseReactor = *p2p.NewBaseReactor("StateSyncReactor", bcSR)
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
		if err := bcSR.pool.Start(); err != nil {
			return err
		}
		go bcSR.poolRoutine()
	} else {
		bcSR.Logger.Info("This node does not need state sync, it might has already state synced, please check", "lockfile", stateSyncLockFileName)
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
			ID:                  StateSyncChannel,
			Priority:            stateSyncPriority,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  1024 * 4096,
			RecvMessageCapacity: maxStateMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our state to peer.
func (bcSR *StateReactor) AddPeer(peer p2p.Peer) {
	// deliberately do nothing when add peer
	// only response when receive bcManifestRequestMessage
	// this is because:
	// 1. unlike block status, snapshot status (manifest) cost more
	// 2. not all nodes in network opened state sync reactor, sending my manifest will cause peers which doesn't
	//    start state sync reactor keep rejecting me - for unknown channel
	bcSR.Logger.Info("added a peer to state sync reactor", "peer", peer.ID())
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcSR *StateReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	bcSR.pool.removePeer(peer.ID())
}

// Receive implements Reactor by handling 4 types of messages (see below).
func (bcSR *StateReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := bcSR.decodeMsg(msgBytes)
	if err != nil {
		bcSR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", fmt.Sprintf("%x", msgBytes))
		bcSR.Switch.StopPeerForError(src, err)
		return
	}

	bcSR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcChunkRequestMessage:
		if queued := bcSR.respondToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
			bcSR.Logger.Error("failed to queue bcChunkResponseMessage", "peer", src.ID(), "req", msg)
		}

	case *bcChunkResponseMessage:
		if bcSR.pool.IsRunning() {
			if snapshotChunk, err := bcSR.validateAndUnmarshalChunk(msg); err == nil {
				if err := bcSR.pool.processChunk(src.ID(), msg, snapshotChunk); err != nil {
					bcSR.Logger.Error("failed process snapshot chunk", "resp", msg, "err", err)
				}
			} else {
				bcSR.Logger.Error("failed validating and decompress snapshot chunk", "resp", msg, "err", err)
				bcSR.Switch.StopPeerForError(src, err)
			}

			// received all chunks!
			if bcSR.pool.isComplete() {
				bcSR.Logger.Info("Time to switch to blockchain reactor!", "height", bcSR.pool.state.LastBlockHeight+1)
				bcSR.startFastSync()
			}
		}

	case *bcNoChunkResponseMessage:
		bcSR.Logger.Error("peer doesn't have state we requested", "peer", src.ID(), "resp", msg)
		bcSR.Switch.StopPeerForError(src, "peer does not have state we requested")

	case *bcManifestRequestMessage:
		if !bcSR.respondManifestToPeer(msg, src) {
			bcSR.Logger.Error("failed to queue bcManifestResponseMessage", "req", msg)
		}

	case *bcManifestResponseMessage:
		if bcSR.pool.IsRunning() {
			if err := bcSR.pool.addManifest(src.ID(), msg); err != nil {
				bcSR.Logger.Error("failed to add manifest", "peer", src.ID(), "err", err)
			}
		}

	default:
		errMsg := fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg))
		bcSR.Logger.Error(errMsg)
		bcSR.Switch.StopPeerForError(src, errMsg)
	}
}

func (bcSR *StateReactor) startFastSync() {
	lockFilePath := filepath.Join(bcSR.config.DBDir(), stateSyncLockFileName)
	if _, err := os.Stat(lockFilePath); os.IsNotExist(err) {
		if _, err := os.Create(lockFilePath); err != nil {
			bcSR.Logger.Error("failed to create state sync lock file", "err", err)
		}
	} else {
		bcSR.Logger.Error("failed to stat state sync lock file", "err", err)
	}

	bcR := bcSR.Switch.Reactor("BLOCKCHAIN").(*blockchain.BlockchainReactor)
	bcR.SwitchToBlockchain(bcSR.pool.state)
	bcSR.pool.Stop()
}

// respondToPeer loads a state and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
// According to the Tendermint spec, if all nodes are honest,
// no node should be requesting for a state that's non-existent.
func (bcSR *StateReactor) respondToPeer(msg *bcChunkRequestMessage,
	src p2p.Peer) (queued bool) {

	if chunk, err := ManagerAt(msg.Height).Reader.Load(msg.Hash); err == nil {
		msg := cdc.MustMarshalBinaryBare(&bcChunkResponseMessage{msg.Hash, chunk})
		return src.TrySend(StateSyncChannel, msg)
	} else {
		bcSR.Logger.Error("failed to load hash for chunk request", "req", msg, "err", err)
		return bcSR.respondNoChunkToPeer(msg, src)
	}
}

func (bcSR *StateReactor) respondManifestToPeer(msg *bcManifestRequestMessage, src p2p.Peer) (queued bool) {
	if height, manifestPayload, err := Manager().Reader.LoadManifest(msg.Height); err == nil {
		// `height` will not equal to `msg.height` when `msg.height == 0`
		msgBytes := cdc.MustMarshalBinaryBare(&bcManifestResponseMessage{height, manifestPayload})
		return src.TrySend(StateSyncChannel, msgBytes)
	} else {
		bcSR.Logger.Error("failed to load manifest, will send an invalid manifest", "height", msg.Height, "err", err)
		msgBytes := cdc.MustMarshalBinaryBare(&bcManifestResponseMessage{0, nil})
		return src.TrySend(StateSyncChannel, msgBytes)
	}
}

func (bcSR *StateReactor) respondNoChunkToPeer(msg *bcChunkRequestMessage, src p2p.Peer) (queued bool) {
	msgBytes := cdc.MustMarshalBinaryBare(&bcNoChunkResponseMessage{msg.Height, msg.Hash})
	return src.TrySend(StateSyncChannel, msgBytes)
}

func (bcSR *StateReactor) validateAndUnmarshalChunk(chunkRespMsg *bcChunkResponseMessage) (abci.SnapshotChunk, error) {
	// check this msg's hash is expected
	if _, ok := bcSR.pool.requesters.Load(chunkRespMsg.Hash); !ok {
		return nil, fmt.Errorf("received a non expected hash %x", chunkRespMsg.Hash)
	}
	// check chunk's hash == msg's hash
	chunkHash := sha256.Sum256(chunkRespMsg.Chunk)
	if chunkRespMsg.Hash != chunkHash {
		return nil, fmt.Errorf("chunk msg's hash %x and calculated chunk %x are not matched", chunkRespMsg.Hash, chunkHash)
	}
	// snappy decode and unmarshal
	if decompressed, err := snappy.Decode(nil, chunkRespMsg.Chunk); err != nil {
		return nil, err
	} else {
		var chunk abci.SnapshotChunk
		err = cdc.UnmarshalBinaryBare(decompressed, &chunk)
		return chunk, err
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (bcSR *StateReactor) poolRoutine() {
	bcSR.BroadcastStateStatusRequest()
	tryInitTicker := time.NewTicker(tryManifestFinalizeSeconds * time.Second)
	defer tryInitTicker.Stop()

	go func() {
		for {
			select {
			case <- bcSR.pool.Quit():
				return

			case <-bcSR.Quit():
				return

			case request := <-bcSR.requestsCh:
				peer := bcSR.Switch.Peers().Get(request.PeerID)
				if peer == nil {
					continue // Peer has since been disconnected.
				}
				msgBytes := cdc.MustMarshalBinaryBare(&bcChunkRequestMessage{request.Height, request.Hash})
				bcSR.Logger.Info("try to request state", "peer", peer.ID(), "req", request)
				queued := peer.TrySend(StateSyncChannel, msgBytes)
				if !queued {
					// We couldn't make the request, send-queue full.
					// The pool handles timeouts, just let it go.
					// TODO: double confirm that spRequster will retry if reactor only missed this particular request
					continue
				}
			}
		}
	}()

FOR_LOOP:
	for {
		select {
		case err := <-bcSR.errorsCh:
			peer := bcSR.Switch.Peers().Get(err.peerID)
			if peer != nil {
				bcSR.Switch.StopPeerForError(peer, err)
			}

		case <-tryInitTicker.C:
			if bcSR.pool.tryInit() {
				if bcSR.pool.manifestHash == bcSR.pool.dummyHash {
					bcSR.Logger.Info("all peers we connected have no manifest, start fast sync directly")
					bcSR.startFastSync()
				}
			}
			// always broadcast state status request to get more potential trusted peers even pool already inited
			bcSR.BroadcastStateStatusRequest()

		case <- bcSR.pool.Quit():
			break FOR_LOOP

		case <-bcSR.Quit():
			break FOR_LOOP
		}
	}
}

// BroadcastStatusRequest broadcasts `StateStore` height.
func (bcSR *StateReactor) BroadcastStateStatusRequest() {
	bcSR.Logger.Info("broadcast state status request")
	msgBytes := cdc.MustMarshalBinaryBare(&bcManifestRequestMessage{})
	bcSR.Switch.Broadcast(StateSyncChannel, msgBytes)
}

//-----------------------------------------------------------------------------
// Messages

// BlockchainStateMessage is a generic message for this reactor.
type BlockchainStateMessage interface{}

func RegisterSnapshotMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*BlockchainStateMessage)(nil), nil)
	cdc.RegisterConcrete(&bcChunkRequestMessage{}, "tendermint/snapshot/bcChunkRequestMessage", nil)
	cdc.RegisterConcrete(&bcChunkResponseMessage{}, "tendermint/snapshot/StateResponse", nil)
	cdc.RegisterConcrete(&bcNoChunkResponseMessage{}, "tendermint/snapshot/NoStateResponse", nil)
	cdc.RegisterConcrete(&bcManifestResponseMessage{}, "tendermint/snapshot/StateStatusResponse", nil)
	cdc.RegisterConcrete(&bcManifestRequestMessage{}, "tendermint/snapshot/ManifestRequest", nil)

	cdc.RegisterConcrete(&abci.Manifest{}, "tendermint/snapshot/Manifest", nil)
	cdc.RegisterInterface((*abci.SnapshotChunk)(nil), nil)
	cdc.RegisterConcrete(&abci.AppStateChunk{}, "tendermint/snapshot/AppStateChunk", nil)
	cdc.RegisterConcrete(&abci.StateChunk{}, "tendermint/snapshot/StateChunk", nil)
	cdc.RegisterConcrete(&abci.BlockChunk{}, "tendermint/snapshot/BlockChunk", nil)
}

func (bcSR *StateReactor) decodeMsg(bz []byte) (msg BlockchainStateMessage, err error) {
	if len(bz) > maxStateMsgSize {
		return nil, fmt.Errorf("state msg exceeds max size (%d > %d)", len(bz), maxStateMsgSize)
	}

	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return msg, err
}

//-------------------------------------

type bcChunkRequestMessage struct {
	Height int64
	Hash abci.SHA256Sum
}

func (m *bcChunkRequestMessage) String() string {
	return fmt.Sprintf("[bcChunkRequestMessage %x(%d)]", m.Hash, m.Height)
}

type bcNoChunkResponseMessage struct {
	Height int64
	Hash abci.SHA256Sum
}

func (m *bcNoChunkResponseMessage) String() string {
	return fmt.Sprintf("[bcNoChunkResponseMessage %x(%d)]", m.Hash, m.Height)
}

//-------------------------------------

type bcChunkResponseMessage struct {
	Hash  abci.SHA256Sum // needed for logging
	Chunk []byte         // compressed of amino encoded chunk
}

func (m *bcChunkResponseMessage) String() string {
	return fmt.Sprintf("[bcChunkResponseMessage %x]", m.Hash)
}

//-------------------------------------

type bcManifestRequestMessage struct {
	Height int64
}

func (m *bcManifestRequestMessage) String() string {
	return fmt.Sprintf("[bcManifestRequestMessage %d]", m.Height)
}

//-------------------------------------

type bcManifestResponseMessage struct {
	Height          int64  // height 0 indicate peer doesn't have manifest matched request
	ManifestPayload []byte // compressed of amino encoded chunk, we don't directly embed Manifest is because we need verify hash of manifests we received from peers in case of fraud manifest
}

func (m *bcManifestResponseMessage) String() string {
	return fmt.Sprintf("[bcManifestResponseMessage %d]", m.Height)
}
