package mock

import (
	"fmt"
	"strings"
	"time"

	dashtypes "github.com/tendermint/tendermint/dash/types"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/mocks"
)

// MOCK SWITCH //
const (
	OpDialMany = "dialMany"
	OpStopOne  = "stopOne"
)

// SwitchHistoryEvent is a log of dial and stop operations executed by the MockSwitch
type SwitchHistoryEvent struct {
	Timestamp time.Time
	Operation string // OpDialMany, OpStopOne
	Params    []string
	Comment   string
}

// Switch implements `dash.Switch`. It sends event about DialPeersAsync() and StopPeerGracefully() calls
// to HistoryChan and stores them in History
type Switch struct {
	PeerSet         *p2p.PeerSet
	PersistentPeers map[string]bool
	History         []SwitchHistoryEvent
	HistoryChan     chan SwitchHistoryEvent
	AddressBook     p2p.AddrBook
}

// NewMockSwitch creates a new mock Switch
func NewMockSwitch() *Switch {
	isw := &Switch{
		PeerSet:         p2p.NewPeerSet(),
		PersistentPeers: map[string]bool{},
		History:         []SwitchHistoryEvent{},
		HistoryChan:     make(chan SwitchHistoryEvent, 1000),
		AddressBook:     &p2p.AddrBookMock{},
	}
	return isw
}

// AddBook returns mock address book to use in this mock switch
func (sw *Switch) AddrBook() p2p.AddrBook {
	return sw.AddressBook
}

// Peers implements Switch by returning sw.PeerSet
func (sw *Switch) Peers() p2p.IPeerSet {
	return sw.PeerSet
}

// AddPersistentPeers implements Switch by marking provided addresses as persistent
func (sw *Switch) AddPersistentPeers(addrs []string) error {
	for _, addr := range addrs {
		addr = simplifyAddress(addr)
		sw.PersistentPeers[addr] = true
	}
	return nil
}

// RemovePersistentPeer implements Switch. It checks if the addr is persistent, and
// marks it as non-persistent if needed.
func (sw Switch) RemovePersistentPeer(addr string) error {
	addr = simplifyAddress(addr)
	if !sw.PersistentPeers[addr] {
		return fmt.Errorf("peer is not persisitent, addr=%s", addr)
	}

	delete(sw.PersistentPeers, addr)
	return nil
}

// DialPeersAsync implements Switch. It emulates connecting to provided addresses
// and adds them as peers and emits history event OpDialMany.
func (sw *Switch) DialPeersAsync(addrs []string) error {
	for _, addr := range addrs {
		peer := &mocks.Peer{}
		parsed, err := dashtypes.ParseValidatorAddress(addr)
		if err != nil {
			return err
		}

		peer.On("ID").Return(parsed.NodeID)
		peer.On("String").Return(addr)
		if err := sw.PeerSet.Add(peer); err != nil {
			return err
		}
	}
	sw.history(OpDialMany, addrs...)
	return nil
}

// IsDialingOrExistingAddress implements Switch. It checks if provided peer has been dialed
// before.
func (sw *Switch) IsDialingOrExistingAddress(addr *p2p.NetAddress) bool {
	return sw.PeerSet.Has(addr.ID)
}

// StopPeerGracefully implements Switch. It removes the peer from Peers() and emits history
// event OpStopOne.
func (sw *Switch) StopPeerGracefully(peer p2p.Peer) {
	sw.PeerSet.Remove(peer)
	sw.history(OpStopOne, peer.String())
}

// history adds info about an operation to sw.History and sends it to sw.HistoryChan
func (sw *Switch) history(op string, args ...string) {
	event := SwitchHistoryEvent{
		Timestamp: time.Now(),
		Operation: op,
		Params:    args,
	}
	sw.History = append(sw.History, event)

	sw.HistoryChan <- event
}

// simplifyAddress converts provided `addr` to a simplified form, to make
// comparisons inside the tests easier
func simplifyAddress(addr string) string {
	return strings.TrimPrefix(addr, "tcp://")
}
