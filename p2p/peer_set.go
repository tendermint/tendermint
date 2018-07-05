package p2p

import (
	"net"
	"sync"
)

// IPeerSet has a (immutable) subset of the methods of PeerSet.
type IPeerSet interface {
	Has(key ID) bool
	HasIP(ip net.IP) bool
	Get(key ID) Peer
	List() []Peer
	Size() int
}

//-----------------------------------------------------------------------------

// PeerSet is a special structure for keeping a table of peers.
// Iteration over the peers is super fast and thread-safe.
type PeerSet struct {
	mtx    sync.Mutex
	lookup map[ID]*peerSetItem
	list   []Peer
}

type peerSetItem struct {
	peer  Peer
	index int
}

// NewPeerSet creates a new peerSet with a list of initial capacity of 256 items.
func NewPeerSet() *PeerSet {
	return &PeerSet{
		lookup: make(map[ID]*peerSetItem),
		list:   make([]Peer, 0, 256),
	}
}

// Add adds the peer to the PeerSet.
// It returns an error carrying the reason, if the peer is already present.
func (ps *PeerSet) Add(peer Peer) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.lookup[peer.ID()] != nil {
		return ErrSwitchDuplicatePeerID{peer.ID()}
	}

	index := len(ps.list)
	// Appending is safe even with other goroutines
	// iterating over the ps.list slice.
	ps.list = append(ps.list, peer)
	ps.lookup[peer.ID()] = &peerSetItem{peer, index}
	return nil
}

// Has returns true if the set contains the peer referred to by this
// peerKey, otherwise false.
func (ps *PeerSet) Has(peerKey ID) bool {
	ps.mtx.Lock()
	_, ok := ps.lookup[peerKey]
	ps.mtx.Unlock()
	return ok
}

// HasIP returns true if the set contains the peer referred to by this IP
// address, otherwise false.
func (ps *PeerSet) HasIP(peerIP net.IP) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.hasIP(peerIP)
}

// hasIP does not acquire a lock so it can be used in public methods which
// already lock.
func (ps *PeerSet) hasIP(peerIP net.IP) bool {
	for _, item := range ps.lookup {
		if item.peer.RemoteIP().Equal(peerIP) {
			return true
		}
	}

	return false
}

// Get looks up a peer by the provided peerKey. Returns nil if peer is not
// found.
func (ps *PeerSet) Get(peerKey ID) Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item, ok := ps.lookup[peerKey]
	if ok {
		return item.peer
	}
	return nil
}

// Remove discards peer by its Key, if the peer was previously memoized.
func (ps *PeerSet) Remove(peer Peer) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	item := ps.lookup[peer.ID()]
	if item == nil {
		return
	}

	index := item.index
	// Create a new copy of the list but with one less item.
	// (we must copy because we'll be mutating the list).
	newList := make([]Peer, len(ps.list)-1)
	copy(newList, ps.list)
	// If it's the last peer, that's an easy special case.
	if index == len(ps.list)-1 {
		ps.list = newList
		delete(ps.lookup, peer.ID())
		return
	}

	// Replace the popped item with the last item in the old list.
	lastPeer := ps.list[len(ps.list)-1]
	lastPeerKey := lastPeer.ID()
	lastPeerItem := ps.lookup[lastPeerKey]
	newList[index] = lastPeer
	lastPeerItem.index = index
	ps.list = newList
	delete(ps.lookup, peer.ID())
}

// Size returns the number of unique items in the peerSet.
func (ps *PeerSet) Size() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return len(ps.list)
}

// List returns the threadsafe list of peers.
func (ps *PeerSet) List() []Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.list
}
