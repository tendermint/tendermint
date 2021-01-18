package mempool

import (
	"errors"
	"fmt"
	"math"
	"time"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	protomem "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	// _ p2p.Wrapper     = (*protomem.Message)(nil)
)

const (
	MempoolChannel = p2p.ChannelID(0x30)

	// peerCatchupSleepIntervalMS defines how much time to sleep if a peer is behind
	peerCatchupSleepIntervalMS = 100

	// UnknownPeerID is the peer ID to use when running CheckTx when there is
	// no peer (e.g. RPC)
	UnknownPeerID uint16 = 0

	maxActiveIDs = math.MaxUint16
)

type mempoolIDs struct {
	mtx       tmsync.RWMutex
	peerMap   map[p2p.NodeID]uint16
	nextID    uint16              // assumes that a node will never have over 65536 active peers
	activeIDs map[uint16]struct{} // used to check if a given peerID key is used, the value doesn't matter
}

// Reactor implements a service that contains mempool of txs that are broadcasted
// amongst peers. It maintains a map from peer ID to counter, to prevent gossiping
// txs to the peers you received it from.
type Reactor struct {
	service.BaseService

	config  *cfg.MempoolConfig
	mempool *CListMempool
	ids     *mempoolIDs
}

// GetChannelShims returns a map of ChannelDescriptorShim objects, where each
// object wraps a reference to a legacy p2p ChannelDescriptor and the corresponding
// p2p proto.Message the new p2p Channel is responsible for handling.
//
//
// TODO: Remove once p2p refactor is complete.
// ref: https://github.com/tendermint/tendermint/issues/5670
func (r *Reactor) GetChannelShims() map[p2p.ChannelID]*p2p.ChannelDescriptorShim {
	largestTx := make([]byte, r.config.MaxTxBytes)
	batchMsg := protomem.Message{
		Sum: &protomem.Message_Txs{
			Txs: &protomem.Txs{Txs: [][]byte{largestTx}},
		},
	}

	return map[p2p.ChannelID]*p2p.ChannelDescriptorShim{
		MempoolChannel: {
			MsgType: new(protomem.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(MempoolChannel),
				Priority:            5,
				RecvMessageCapacity: batchMsg.Size(),
			},
		},
	}
}

// ============================================================================

// Reserve searches for the next unused ID and assigns it to the
// peer.
func (ids *mempoolIDs) ReserveForPeer(peer p2p.Peer) {
	ids.mtx.Lock()
	defer ids.mtx.Unlock()

	curID := ids.nextPeerID()
	ids.peerMap[peer.ID()] = curID
	ids.activeIDs[curID] = struct{}{}
}

// nextPeerID returns the next unused peer ID to use.
// This assumes that ids's mutex is already locked.
func (ids *mempoolIDs) nextPeerID() uint16 {
	if len(ids.activeIDs) == maxActiveIDs {
		panic(fmt.Sprintf("node has maximum %d active IDs and wanted to get one more", maxActiveIDs))
	}

	_, idExists := ids.activeIDs[ids.nextID]
	for idExists {
		ids.nextID++
		_, idExists = ids.activeIDs[ids.nextID]
	}
	curID := ids.nextID
	ids.nextID++
	return curID
}

// Reclaim returns the ID reserved for the peer back to unused pool.
func (ids *mempoolIDs) Reclaim(peer p2p.Peer) {
	ids.mtx.Lock()
	defer ids.mtx.Unlock()

	removedID, ok := ids.peerMap[peer.ID()]
	if ok {
		delete(ids.activeIDs, removedID)
		delete(ids.peerMap, peer.ID())
	}
}

// GetForPeer returns an ID reserved for the peer.
func (ids *mempoolIDs) GetForPeer(peer p2p.Peer) uint16 {
	ids.mtx.RLock()
	defer ids.mtx.RUnlock()

	return ids.peerMap[peer.ID()]
}

func newMempoolIDs() *mempoolIDs {
	return &mempoolIDs{
		peerMap:   make(map[p2p.NodeID]uint16),
		activeIDs: map[uint16]struct{}{0: {}},
		nextID:    1, // reserve unknownPeerID(0) for mempoolReactor.BroadcastTx
	}
}

// NewReactor returns a new Reactor with the given config and mempool.
func NewReactor(config *cfg.MempoolConfig, mempool *CListMempool) *Reactor {
	memR := &Reactor{
		config:  config,
		mempool: mempool,
		ids:     newMempoolIDs(),
	}
	memR.BaseReactor = *p2p.NewBaseReactor("Mempool", memR)
	return memR
}

// InitPeer implements Reactor by creating a state for the peer.
func (memR *Reactor) InitPeer(peer p2p.Peer) p2p.Peer {
	memR.ids.ReserveForPeer(peer)
	return peer
}

// SetLogger sets the Logger on the reactor and the underlying mempool.
func (memR *Reactor) SetLogger(l log.Logger) {
	memR.Logger = l
	memR.mempool.SetLogger(l)
}

// OnStart implements p2p.BaseReactor.
func (memR *Reactor) OnStart() error {
	if !memR.config.Broadcast {
		memR.Logger.Info("Tx broadcasting is disabled")
	}
	return nil
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (memR *Reactor) AddPeer(peer p2p.Peer) {
	if memR.config.Broadcast {
		go memR.broadcastTxRoutine(peer)
	}
}

// RemovePeer implements Reactor.
func (memR *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	memR.ids.Reclaim(peer)
	// broadcast routine checks if peer is gone and returns
}

// Receive implements Reactor.
// It adds any received transactions to the mempool.
// XXX: do not call any methods that can block or incur heavy processing.
// https://github.com/tendermint/tendermint/issues/2888
func (memR *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := memR.decodeMsg(msgBytes)
	if err != nil {
		memR.Logger.Error("Error decoding message", "src", src, "chId", chID, "err", err)
		memR.Switch.StopPeerForError(src, err)
		return
	}
	memR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	txInfo := TxInfo{SenderID: memR.ids.GetForPeer(src)}
	if src != nil {
		txInfo.SenderP2PID = src.ID()
	}
	for _, tx := range msg.Txs {
		err = memR.mempool.CheckTx(tx, nil, txInfo)
		if err != nil {
			memR.Logger.Info("Could not check tx", "tx", txID(tx), "err", err)
		}
	}
	// broadcasting happens from go routines per peer
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() int64
}

// Send new mempool txs to peer.
func (memR *Reactor) broadcastTxRoutine(peer p2p.Peer) {
	peerID := memR.ids.GetForPeer(peer)
	var next *clist.CElement

	for {
		// In case of both next.NextWaitChan() and peer.Quit() are variable at the same time
		if !memR.IsRunning() || !peer.IsRunning() {
			return
		}
		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if next == nil {
			select {
			case <-memR.mempool.TxsWaitChan(): // Wait until a tx is available
				if next = memR.mempool.TxsFront(); next == nil {
					continue
				}
			case <-peer.Quit():
				return
			case <-memR.Quit():
				return
			}
		}

		// Make sure the peer is up to date.
		peerState, ok := peer.Get(types.PeerStateKey).(PeerState)
		if !ok {
			// Peer does not have a state yet. We set it in the consensus reactor, but
			// when we add peer in Switch, the order we call reactors#AddPeer is
			// different every time due to us using a map. Sometimes other reactors
			// will be initialized before the consensus reactor. We should wait a few
			// milliseconds and retry.
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}

		// Allow for a lag of 1 block.
		memTx := next.Value.(*mempoolTx)
		if peerState.GetHeight() < memTx.Height()-1 {
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}

		// NOTE: Transaction batching was disabled due to
		// https://github.com/tendermint/tendermint/issues/5796

		if _, ok := memTx.senders.Load(peerID); !ok {
			msg := protomem.Message{
				Sum: &protomem.Message_Txs{
					Txs: &protomem.Txs{Txs: [][]byte{memTx.tx}},
				},
			}
			bz, err := msg.Marshal()
			if err != nil {
				panic(err)
			}
			success := peer.Send(MempoolChannel, bz)
			if !success {
				time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
				continue
			}
		}

		select {
		case <-next.NextWaitChan():
			// see the start of the for loop for nil check
			next = next.Next()
		case <-peer.Quit():
			return
		case <-memR.Quit():
			return
		}
	}
}

//-----------------------------------------------------------------------------
// Messages

func (memR *Reactor) decodeMsg(bz []byte) (TxsMessage, error) {
	msg := protomem.Message{}
	err := msg.Unmarshal(bz)
	if err != nil {
		return TxsMessage{}, err
	}

	var message TxsMessage

	if i, ok := msg.Sum.(*protomem.Message_Txs); ok {
		txs := i.Txs.GetTxs()

		if len(txs) == 0 {
			return message, errors.New("empty TxsMessage")
		}

		decoded := make([]types.Tx, len(txs))
		for j, tx := range txs {
			decoded[j] = types.Tx(tx)
		}

		message = TxsMessage{
			Txs: decoded,
		}
		return message, nil
	}
	return message, fmt.Errorf("msg type: %T is not supported", msg)
}

//-------------------------------------

// TxsMessage is a Message containing transactions.
type TxsMessage struct {
	Txs []types.Tx
}

// String returns a string representation of the TxsMessage.
func (m *TxsMessage) String() string {
	return fmt.Sprintf("[TxsMessage %v]", m.Txs)
}
