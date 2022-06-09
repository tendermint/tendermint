package mock

import (
	"encoding/binary"
	"encoding/hex"
	"sync"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	OpDial = "dialOne"
	OpStop = "stopOne"
)

// HistoryEvent is a log of dial and stop operations executed by the DashDialer
type HistoryEvent struct {
	Operation string // OpDialMany, OpStopOne
	Params    []string
}

// DashDialer is a mock `p2p.DashDialer`.
// It sends event about DialPeersAsync() and StopPeerGracefully() calls
// to HistoryChan and stores them in History
type DashDialer struct {
	mux            sync.Mutex
	ConnectedPeers map[types.NodeID]bool
	HistoryChan    chan HistoryEvent
}

// NewDashDialer creates a new mock p2p.DashDialer that sends
// notifications on all events to HistoryChan channel.
func NewDashDialer() *DashDialer {
	isw := &DashDialer{
		ConnectedPeers: map[types.NodeID]bool{},
		HistoryChan:    make(chan HistoryEvent, 1000),
	}
	return isw
}

// ConnectAsync implements p2p.DashDialer.
// It emulates connecting to provided address, adds is as a connected peer
// and emits history event OpDial.
func (sw *DashDialer) ConnectAsync(addr p2p.NodeAddress) error {
	id := addr.NodeID
	sw.mux.Lock()
	sw.ConnectedPeers[id] = true
	sw.mux.Unlock()

	sw.history(OpDial, string(id))
	return nil
}

// IsDialingOrConnected implements p2p.DashDialer.
// It checks if provided peer is connected or dial is in progress.
func (sw *DashDialer) IsDialingOrConnected(id types.NodeID) bool {
	sw.mux.Lock()
	defer sw.mux.Unlock()
	return sw.ConnectedPeers[id]
}

// DisconnectAsync implements p2p.DashDialer.
// It removes the peer from list of connected peers and emits history
// event OpStop
func (sw *DashDialer) DisconnectAsync(id types.NodeID) error {
	sw.mux.Lock()
	sw.ConnectedPeers[id] = false
	sw.mux.Unlock()
	sw.history(OpStop, string(id))

	return nil
}
func (sw *DashDialer) Resolve(val types.ValidatorAddress) (p2p.NodeAddress, error) {
	// Generate node ID
	nodeID := make([]byte, 20)
	n := val.Port

	binary.LittleEndian.PutUint64(nodeID, uint64(n))

	addr := p2p.NodeAddress{
		NodeID:   types.NodeID(hex.EncodeToString(nodeID)),
		Protocol: p2p.TCPProtocol,
		Hostname: val.Hostname,
		Port:     val.Port,
		Path:     "",
	}

	return addr, nil
}

// history adds info about an operation to sw.HistoryChan
func (sw *DashDialer) history(op string, args ...string) {
	event := HistoryEvent{
		Operation: op,
		Params:    args,
	}
	sw.HistoryChan <- event
}
