package p2p

import (
	"sync"
)

/*
PeerSet is a special structure for keeping a table of peers.
Iteration over the peers is super fast and thread-safe.
*/
type PeerSet struct {
	mtx    sync.Mutex
	lookup map[string]*peerSetItem
	list   []*Peer
}

type peerSetItem struct {
	peer  *Peer
	index int
}

func NewPeerSet() *PeerSet {
	return &PeerSet{
		lookup: make(map[string]*peerSetItem),
		list:   make([]*Peer, 0, 256),
	}
}

func (ps *PeerSet) Add(peer *Peer) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	addr := peer.RemoteAddress().String()
	if ps.lookup[addr] != nil {
		return false
	}
	index := len(ps.list)
	// Appending is safe even with other goroutines
	// iterating over the ps.list slice.
	ps.list = append(ps.list, peer)
	ps.lookup[addr] = &peerSetItem{peer, index}
	return true
}

func (ps *PeerSet) Remove(peer *Peer) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	addr := peer.RemoteAddress().String()
	item := ps.lookup[addr]
	if item == nil {
		return
	}
	index := item.index
	// If it's the last peer, that's an easy special case.
	if index == len(ps.list)-1 {
		ps.list = ps.list[:len(ps.list)-1]
		return
	}
	// Copy the list but without the last element.
	newList := make([]*Peer, len(ps.list)-1)
	copy(newList, ps.list)
	// Move the last item from ps.list to "index" in list.
	lastPeer := ps.list[len(ps.list)-1]
	lastPeerAddr := lastPeer.RemoteAddress().String()
	lastPeerItem := ps.lookup[lastPeerAddr]
	newList[index] = lastPeer
	lastPeerItem.index = index
	ps.list = newList
	delete(ps.lookup, addr)
}

// threadsafe list of peers.
func (ps *PeerSet) List() []*Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.list
}
