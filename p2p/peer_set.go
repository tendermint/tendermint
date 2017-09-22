// Copyright 2017 Tendermint. All Rights Reserved.
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

package p2p

import (
	"sync"
)

// IPeerSet has a (immutable) subset of the methods of PeerSet.
type IPeerSet interface {
	Has(key string) bool
	Get(key string) Peer
	List() []Peer
	Size() int
}

//-----------------------------------------------------------------------------

// PeerSet is a special structure for keeping a table of peers.
// Iteration over the peers is super fast and thread-safe.
type PeerSet struct {
	mtx    sync.Mutex
	lookup map[string]*peerSetItem
	list   []Peer
}

type peerSetItem struct {
	peer  Peer
	index int
}

// NewPeerSet creates a new peerSet with a list of initial capacity of 256 items.
func NewPeerSet() *PeerSet {
	return &PeerSet{
		lookup: make(map[string]*peerSetItem),
		list:   make([]Peer, 0, 256),
	}
}

// Add adds the peer to the PeerSet.
// It returns ErrSwitchDuplicatePeer if the peer is already present.
func (ps *PeerSet) Add(peer Peer) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if ps.lookup[peer.Key()] != nil {
		return ErrSwitchDuplicatePeer
	}

	index := len(ps.list)
	// Appending is safe even with other goroutines
	// iterating over the ps.list slice.
	ps.list = append(ps.list, peer)
	ps.lookup[peer.Key()] = &peerSetItem{peer, index}
	return nil
}

// Has returns true iff the PeerSet contains
// the peer referred to by this peerKey.
func (ps *PeerSet) Has(peerKey string) bool {
	ps.mtx.Lock()
	_, ok := ps.lookup[peerKey]
	ps.mtx.Unlock()
	return ok
}

// Get looks up a peer by the provided peerKey.
func (ps *PeerSet) Get(peerKey string) Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item, ok := ps.lookup[peerKey]
	if ok {
		return item.peer
	} else {
		return nil
	}
}

// Remove discards peer by its Key, if the peer was previously memoized.
func (ps *PeerSet) Remove(peer Peer) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item := ps.lookup[peer.Key()]
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
		delete(ps.lookup, peer.Key())
		return
	}

	// Replace the popped item with the last item in the old list.
	lastPeer := ps.list[len(ps.list)-1]
	lastPeerKey := lastPeer.Key()
	lastPeerItem := ps.lookup[lastPeerKey]
	newList[index] = lastPeer
	lastPeerItem.index = index
	ps.list = newList
	delete(ps.lookup, peer.Key())
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
