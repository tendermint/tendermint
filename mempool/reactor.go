package mempool

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	amino "github.com/tendermint/go-amino"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	MempoolChannel = byte(0x30)

	maxMsgSize                 = 1048576 // 1MB TODO make it configurable
	peerCatchupSleepIntervalMS = 100     // If peer is behind, sleep this amount
	unknownPeerID              = 0
)

// MempoolReactor handles mempool tx broadcasting amongst peers.
// It maintains a map from peer ID to counter, to prevent gossiping txs to the
// peers you received it from.
type MempoolReactor struct {
	p2p.BaseReactor
	config  *cfg.MempoolConfig
	Mempool *Mempool

	idMtx   *sync.RWMutex
	nextID  uint16 // assumes that a node will never have over 65536 peers
	peerMap map[p2p.ID]uint16
}

// NewMempoolReactor returns a new MempoolReactor with the given config and mempool.
func NewMempoolReactor(config *cfg.MempoolConfig, mempool *Mempool) *MempoolReactor {
	memR := &MempoolReactor{
		config:  config,
		Mempool: mempool,

		idMtx:   &sync.RWMutex{},
		peerMap: make(map[p2p.ID]uint16),
		nextID:  1, // reserve unknownPeerID(0) for mempoolReactor.BroadcastTx
	}
	memR.BaseReactor = *p2p.NewBaseReactor("MempoolReactor", memR)
	return memR
}

// SetLogger sets the Logger on the reactor and the underlying Mempool.
func (memR *MempoolReactor) SetLogger(l log.Logger) {
	memR.Logger = l
	memR.Mempool.SetLogger(l)
}

// OnStart implements p2p.BaseReactor.
func (memR *MempoolReactor) OnStart() error {
	if !memR.config.Broadcast {
		memR.Logger.Info("Tx broadcasting is disabled")
	}
	return nil
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (memR *MempoolReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:       MempoolChannel,
			Priority: 5,
		},
	}
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (memR *MempoolReactor) AddPeer(peer p2p.Peer) {
	memR.idMtx.Lock()
	memR.peerMap[peer.ID()] = memR.nextID
	memR.nextID++
	memR.idMtx.Unlock()
	go memR.broadcastTxRoutine(peer)
}

// RemovePeer implements Reactor.
func (memR *MempoolReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	memR.idMtx.Lock()
	delete(memR.peerMap, peer.ID())
	memR.idMtx.Unlock()
	// broadcast routine checks if peer is gone and returns
}

// Receive implements Reactor.
// It adds any received transactions to the mempool.
func (memR *MempoolReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		memR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		memR.Switch.StopPeerForError(src, err)
		return
	}
	memR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *TxMessage:
		memR.idMtx.RLock()
		peerID := memR.peerMap[src.ID()]
		memR.idMtx.RUnlock()
		err := memR.Mempool.checkTxFromPeer(msg.Tx, nil, peerID)
		if err != nil {
			memR.Logger.Info("Could not check tx", "tx", TxID(msg.Tx), "err", err)
		}
		// broadcasting happens from go routines per peer
	default:
		memR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// BroadcastTx is an alias for Mempool.CheckTx. Broadcasting itself happens in peer routines.
func (memR *MempoolReactor) BroadcastTx(tx types.Tx, cb func(*abci.Response)) error {
	return memR.Mempool.CheckTx(tx, cb)
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() int64
}

// Send new mempool txs to peer.
func (memR *MempoolReactor) broadcastTxRoutine(peer p2p.Peer) {
	if !memR.config.Broadcast {
		return
	}

	memR.idMtx.RLock()
	peerID := memR.peerMap[peer.ID()]
	memR.idMtx.RUnlock()
	var next *clist.CElement
	for {
		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if next == nil {
			select {
			case <-memR.Mempool.TxsWaitChan(): // Wait until a tx is available
				if next = memR.Mempool.TxsFront(); next == nil {
					continue
				}
			case <-peer.Quit():
				return
			case <-memR.Quit():
				return
			}
		}

		memTx := next.Value.(*mempoolTx)

		// make sure the peer is up to date
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
		if peerState.GetHeight() < memTx.Height()-1 { // Allow for a lag of 1 block
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}
		// ensure peer hasn't already sent us this tx
		skip := false
		for i := 0; i < len(memTx.senders); i++ {
			if peerID == memTx.senders[i] {
				skip = true
				break
			}
		}

		if !skip {
			// send memTx
			msg := &TxMessage{Tx: memTx.tx}
			success := peer.Send(MempoolChannel, cdc.MustMarshalBinaryBare(msg))
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

// MempoolMessage is a message sent or received by the MempoolReactor.
type MempoolMessage interface{}

func RegisterMempoolMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*MempoolMessage)(nil), nil)
	cdc.RegisterConcrete(&TxMessage{}, "tendermint/mempool/TxMessage", nil)
}

func decodeMsg(bz []byte) (msg MempoolMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------

// TxMessage is a MempoolMessage containing a transaction.
type TxMessage struct {
	Tx types.Tx
}

// String returns a string representation of the TxMessage.
func (m *TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}
