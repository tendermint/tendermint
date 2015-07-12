package p2p

import (
	"net"
	"strings"
	"sync"
)

// IPeerSet has a (immutable) subset of the methods of PeerSet.
type IPeerSet interface {
	Has(key string) bool
	Get(key string) *Peer
	List() []*Peer
	Size() int
}

//-----------------------------------------------------------------------------

var (
	maxPeersPerIPRange = [4]int{11, 7, 5, 3} // ...
)

// PeerSet is a special structure for keeping a table of peers.
// Iteration over the peers is super fast and thread-safe.
// We also track how many peers per ip range and avoid too many
type PeerSet struct {
	mtx          sync.Mutex
	lookup       map[string]*peerSetItem
	list         []*Peer
	connectedIPs *nestedCounter
}

type peerSetItem struct {
	peer  *Peer
	index int
}

func NewPeerSet() *PeerSet {
	return &PeerSet{
		lookup:       make(map[string]*peerSetItem),
		list:         make([]*Peer, 0, 256),
		connectedIPs: NewNestedCounter(),
	}
}

// Returns false if peer with key (uuid) is already in set
// or if we have too many peers from the peer's ip range
func (ps *PeerSet) Add(peer *Peer) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if ps.lookup[peer.Key] != nil {
		return ErrSwitchDuplicatePeer
	}

	// ensure we havent maxed out connections for the peer's ip range yet
	// and update the ip range counters
	if !ps.updateIPRangeCounts(peer.Host) {
		return ErrSwitchMaxPeersPerIPRange
	}

	index := len(ps.list)
	// Appending is safe even with other goroutines
	// iterating over the ps.list slice.
	ps.list = append(ps.list, peer)
	ps.lookup[peer.Key] = &peerSetItem{peer, index}
	return nil
}

func (ps *PeerSet) Has(peerKey string) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	_, ok := ps.lookup[peerKey]
	return ok
}

func (ps *PeerSet) Get(peerKey string) *Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item, ok := ps.lookup[peerKey]
	if ok {
		return item.peer
	} else {
		return nil
	}
}

func (ps *PeerSet) Remove(peer *Peer) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item := ps.lookup[peer.Key]
	if item == nil {
		return
	}
	index := item.index
	// Copy the list but without the last element.
	// (we must copy because we're mutating the list)
	newList := make([]*Peer, len(ps.list)-1)
	copy(newList, ps.list)
	// If it's the last peer, that's an easy special case.
	if index == len(ps.list)-1 {
		ps.list = newList
		delete(ps.lookup, peer.Key)
		return
	}
	// Move the last item from ps.list to "index" in list.
	lastPeer := ps.list[len(ps.list)-1]
	lastPeerKey := lastPeer.Key
	lastPeerItem := ps.lookup[lastPeerKey]
	newList[index] = lastPeer
	lastPeerItem.index = index
	ps.list = newList
	delete(ps.lookup, peer.Key)
}

func (ps *PeerSet) Size() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return len(ps.list)
}

// threadsafe list of peers.
func (ps *PeerSet) List() []*Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.list
}

//-----------------------------------------------------------------------------
// track the number of ips we're connected to for each ip address range

// forms an ip address hierarchy tree with counts
// the struct itself is not thread safe and should always only be accessed with the ps.mtx locked
type nestedCounter struct {
	count    int
	children map[string]*nestedCounter
}

func NewNestedCounter() *nestedCounter {
	nc := new(nestedCounter)
	nc.children = make(map[string]*nestedCounter)
	return nc
}

// Check if we have too many ips in the ip range of the incoming connection
// Thread safe
func (ps *PeerSet) HasMaxForIPRange(conn net.Conn) (ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	spl := strings.Split(ip, ".")

	c := ps.connectedIPs
	for i, ipByte := range spl {
		if c, ok = c.children[ipByte]; !ok {
			return false
		}
		if c.count == maxPeersPerIPRange[i] {
			return true
		}
	}
	return false
}

// Update counts for this address' ip range
// Returns false if we already have enough connections
// Not thread safe (only called by ps.Add())
func (ps *PeerSet) updateIPRangeCounts(address string) bool {
	spl := strings.Split(address, ".")

	c := ps.connectedIPs
	return updateNestedCountRecursive(c, spl, 0)
}

// recursively descend the ip hierarchy, checking if we have
// max peers for each range and updating if not
func updateNestedCountRecursive(c *nestedCounter, ipBytes []string, index int) bool {
	if index == len(ipBytes) {
		return true
	}
	ipByte := ipBytes[index]
	if c2, ok := c.children[ipByte]; !ok {
		c2 = NewNestedCounter()
		c.children[ipByte] = c2
		c = c2
	} else {
		c = c2
		if c.count == maxPeersPerIPRange[index] {
			return false
		}
	}
	if !updateNestedCountRecursive(c, ipBytes, index+1) {
		return false
	}
	c.count += 1
	return true
}
