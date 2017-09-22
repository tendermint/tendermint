// Copyright 2014 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mempool

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	abci "github.com/tendermint/abci/types"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tmlibs/clist"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	MempoolChannel = byte(0x30)

	maxMempoolMessageSize      = 1048576 // 1MB TODO make it configurable
	peerCatchupSleepIntervalMS = 100     // If peer is behind, sleep this amount
)

// MempoolReactor handles mempool tx broadcasting amongst peers.
type MempoolReactor struct {
	p2p.BaseReactor
	config  *cfg.MempoolConfig
	Mempool *Mempool
	evsw    types.EventSwitch
}

// NewMempoolReactor returns a new MempoolReactor with the given config and mempool.
func NewMempoolReactor(config *cfg.MempoolConfig, mempool *Mempool) *MempoolReactor {
	memR := &MempoolReactor{
		config:  config,
		Mempool: mempool,
	}
	memR.BaseReactor = *p2p.NewBaseReactor("MempoolReactor", memR)
	return memR
}

// SetLogger sets the Logger on the reactor and the underlying Mempool.
func (memR *MempoolReactor) SetLogger(l log.Logger) {
	memR.Logger = l
	memR.Mempool.SetLogger(l)
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (memR *MempoolReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			ID:       MempoolChannel,
			Priority: 5,
		},
	}
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (memR *MempoolReactor) AddPeer(peer p2p.Peer) {
	go memR.broadcastTxRoutine(peer)
}

// RemovePeer implements Reactor.
func (memR *MempoolReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	// broadcast routine checks if peer is gone and returns
}

// Receive implements Reactor.
// It adds any received transactions to the mempool.
func (memR *MempoolReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		memR.Logger.Error("Error decoding message", "err", err)
		return
	}
	memR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *TxMessage:
		err := memR.Mempool.CheckTx(msg.Tx, nil)
		if err != nil {
			memR.Logger.Info("Could not check tx", "tx", msg.Tx, "err", err)
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
	GetHeight() int
}

// Peer describes a peer.
type Peer interface {
	IsRunning() bool
	Send(byte, interface{}) bool
	Get(string) interface{}
}

// Send new mempool txs to peer.
// TODO: Handle mempool or reactor shutdown?
// As is this routine may block forever if no new txs come in.
func (memR *MempoolReactor) broadcastTxRoutine(peer Peer) {
	if !memR.config.Broadcast {
		return
	}

	var next *clist.CElement
	for {
		if !memR.IsRunning() || !peer.IsRunning() {
			return // Quit!
		}
		if next == nil {
			// This happens because the CElement we were looking at got
			// garbage collected (removed).  That is, .NextWait() returned nil.
			// Go ahead and start from the beginning.
			next = memR.Mempool.TxsFrontWait() // Wait until a tx is available
		}
		memTx := next.Value.(*mempoolTx)
		// make sure the peer is up to date
		height := memTx.Height()
		if peerState_i := peer.Get(types.PeerStateKey); peerState_i != nil {
			peerState := peerState_i.(PeerState)
			if peerState.GetHeight() < height-1 { // Allow for a lag of 1 block
				time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
				continue
			}
		}
		// send memTx
		msg := &TxMessage{Tx: memTx.tx}
		success := peer.Send(MempoolChannel, struct{ MempoolMessage }{msg})
		if !success {
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}

		next = next.NextWait()
		continue
	}
}

// SetEventSwitch implements events.Eventable.
func (memR *MempoolReactor) SetEventSwitch(evsw types.EventSwitch) {
	memR.evsw = evsw
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeTx = byte(0x01)
)

// MempoolMessage is a message sent or received by the MempoolReactor.
type MempoolMessage interface{}

var _ = wire.RegisterInterface(
	struct{ MempoolMessage }{},
	wire.ConcreteType{&TxMessage{}, msgTypeTx},
)

// DecodeMessage decodes a byte-array into a MempoolMessage.
func DecodeMessage(bz []byte) (msgType byte, msg MempoolMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ MempoolMessage }{}, r, maxMempoolMessageSize, n, &err).(struct{ MempoolMessage }).MempoolMessage
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
