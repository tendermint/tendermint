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
// We also track how many peers per IP range and avoid too many
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

// Returns false if peer with key (PubKeyEd25519) is already in set
// or if we have too many peers from the peer's IP range
func (ps *PeerSet) Add(peer *Peer) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	if ps.lookup[peer.Key] != nil {
		return ErrSwitchDuplicatePeer
	}

	// ensure we havent maxed out connections for the peer's IP range yet
	// and update the IP range counters
	if !ps.incrIPRangeCounts(peer.Host) {
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

	// update the IP range counters
	ps.decrIPRangeCounts(peer.Host)

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
// track the number of IPs we're connected to for each IP address range

// forms an IP address hierarchy tree with counts
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

// Check if we have too many IPs in the IP range of the incoming connection
// Thread safe
func (ps *PeerSet) HasMaxForIPRange(conn net.Conn) (ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	ipBytes := strings.Split(ip, ".")

	c := ps.connectedIPs
	for i, ipByte := range ipBytes {
		if c, ok = c.children[ipByte]; !ok {
			return false
		}
		if maxPeersPerIPRange[i] <= c.count {
			return true
		}
	}
	return false
}

// Increments counts for this address' IP range
// Returns false if we already have enough connections
// Not thread safe (only called by ps.Add())
func (ps *PeerSet) incrIPRangeCounts(address string) bool {
	addrParts := strings.Split(address, ".")

	c := ps.connectedIPs
	return incrNestedCounters(c, addrParts, 0)
}

// Recursively descend the IP hierarchy, checking if we have
// max peers for each range and incrementing if not.
// Returns false if incr failed because max peers reached for some range counter.
func incrNestedCounters(c *nestedCounter, ipBytes []string, index int) bool {
	ipByte := ipBytes[index]
	child := c.children[ipByte]
	if child == nil {
		child = NewNestedCounter()
		c.children[ipByte] = child
	}
	if index+1 < len(ipBytes) {
		if !incrNestedCounters(child, ipBytes, index+1) {
			return false
		}
	}
	if maxPeersPerIPRange[index] <= child.count {
		return false
	} else {
		child.count += 1
		return true
	}
}

// Decrement counts for this address' IP range
func (ps *PeerSet) decrIPRangeCounts(address string) {
	addrParts := strings.Split(address, ".")

	c := ps.connectedIPs
	decrNestedCounters(c, addrParts, 0)
}

// Recursively descend the IP hierarchy, decrementing by one.
// If the counter is zero, deletes the child.
func decrNestedCounters(c *nestedCounter, ipBytes []string, index int) {
	ipByte := ipBytes[index]
	child := c.children[ipByte]
	if child == nil {
		log.Error("p2p/peer_set decrNestedCounters encountered a missing child counter")
		return
	}
	if index+1 < len(ipBytes) {
		decrNestedCounters(child, ipBytes, index+1)
	}
	child.count -= 1
	if child.count <= 0 {
		delete(c.children, ipByte)
	}
}
