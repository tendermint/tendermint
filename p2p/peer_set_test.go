package p2p

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
)

// mockPeer for testing the PeerSet
type mockPeer struct {
	cmn.BaseService
	ip net.IP
	id ID
}

func (mp *mockPeer) FlushStop()                              { mp.Stop() }
func (mp *mockPeer) TrySend(chID byte, msgBytes []byte) bool { return true }
func (mp *mockPeer) Send(chID byte, msgBytes []byte) bool    { return true }
func (mp *mockPeer) NodeInfo() NodeInfo                      { return DefaultNodeInfo{} }
func (mp *mockPeer) Status() ConnectionStatus                { return ConnectionStatus{} }
func (mp *mockPeer) ID() ID                                  { return mp.id }
func (mp *mockPeer) IsOutbound() bool                        { return false }
func (mp *mockPeer) IsPersistent() bool                      { return true }
func (mp *mockPeer) Get(s string) interface{}                { return s }
func (mp *mockPeer) Set(string, interface{})                 {}
func (mp *mockPeer) RemoteIP() net.IP                        { return mp.ip }
func (mp *mockPeer) SocketAddr() *NetAddress                 { return nil }
func (mp *mockPeer) RemoteAddr() net.Addr                    { return &net.TCPAddr{IP: mp.ip, Port: 8800} }
func (mp *mockPeer) CloseConn() error                        { return nil }

// Returns a mock peer
func newMockPeer(ip net.IP) *mockPeer {
	if ip == nil {
		ip = net.IP{127, 0, 0, 1}
	}
	nodeKey := NodeKey{PrivKey: ed25519.GenPrivKey()}
	return &mockPeer{
		ip: ip,
		id: nodeKey.ID(),
	}
}

func TestPeerSetAddRemoveOne(t *testing.T) {
	t.Parallel()

	peerSet := NewPeerSet()

	var peerList []Peer
	for i := 0; i < 5; i++ {
		p := newMockPeer(net.IP{127, 0, 0, byte(i)})
		if err := peerSet.Add(p); err != nil {
			t.Error(err)
		}
		peerList = append(peerList, p)
	}

	n := len(peerList)
	// 1. Test removing from the front
	for i, peerAtFront := range peerList {
		removed := peerSet.Remove(peerAtFront)
		assert.True(t, removed)
		wantSize := n - i - 1
		for j := 0; j < 2; j++ {
			assert.Equal(t, false, peerSet.Has(peerAtFront.ID()), "#%d Run #%d: failed to remove peer", i, j)
			assert.Equal(t, wantSize, peerSet.Size(), "#%d Run #%d: failed to remove peer and decrement size", i, j)
			// Test the route of removing the now non-existent element
			removed := peerSet.Remove(peerAtFront)
			assert.False(t, removed)
		}
	}

	// 2. Next we are testing removing the peer at the end
	// a) Replenish the peerSet
	for _, peer := range peerList {
		if err := peerSet.Add(peer); err != nil {
			t.Error(err)
		}
	}

	// b) In reverse, remove each element
	for i := n - 1; i >= 0; i-- {
		peerAtEnd := peerList[i]
		removed := peerSet.Remove(peerAtEnd)
		assert.True(t, removed)
		assert.Equal(t, false, peerSet.Has(peerAtEnd.ID()), "#%d: failed to remove item at end", i)
		assert.Equal(t, i, peerSet.Size(), "#%d: differing sizes after peerSet.Remove(atEndPeer)", i)
	}
}

func TestPeerSetAddRemoveMany(t *testing.T) {
	t.Parallel()
	peerSet := NewPeerSet()

	peers := []Peer{}
	N := 100
	for i := 0; i < N; i++ {
		peer := newMockPeer(net.IP{127, 0, 0, byte(i)})
		if err := peerSet.Add(peer); err != nil {
			t.Errorf("Failed to add new peer")
		}
		if peerSet.Size() != i+1 {
			t.Errorf("Failed to add new peer and increment size")
		}
		peers = append(peers, peer)
	}

	for i, peer := range peers {
		removed := peerSet.Remove(peer)
		assert.True(t, removed)
		if peerSet.Has(peer.ID()) {
			t.Errorf("Failed to remove peer")
		}
		if peerSet.Size() != len(peers)-i-1 {
			t.Errorf("Failed to remove peer and decrement size")
		}
	}
}

func TestPeerSetAddDuplicate(t *testing.T) {
	t.Parallel()
	peerSet := NewPeerSet()
	peer := newMockPeer(nil)

	n := 20
	errsChan := make(chan error)
	// Add the same asynchronously to test the
	// concurrent guarantees of our APIs, and
	// our expectation in the end is that only
	// one addition succeeded, but the rest are
	// instances of ErrSwitchDuplicatePeer.
	for i := 0; i < n; i++ {
		go func() {
			errsChan <- peerSet.Add(peer)
		}()
	}

	// Now collect and tally the results
	errsTally := make(map[string]int)
	for i := 0; i < n; i++ {
		err := <-errsChan

		switch err.(type) {
		case ErrSwitchDuplicatePeerID:
			errsTally["duplicateID"]++
		default:
			errsTally["other"]++
		}
	}

	// Our next procedure is to ensure that only one addition
	// succeeded and that the rest are each ErrSwitchDuplicatePeer.
	wantErrCount, gotErrCount := n-1, errsTally["duplicateID"]
	assert.Equal(t, wantErrCount, gotErrCount, "invalid ErrSwitchDuplicatePeer count")

	wantNilErrCount, gotNilErrCount := 1, errsTally["other"]
	assert.Equal(t, wantNilErrCount, gotNilErrCount, "invalid nil errCount")
}

func TestPeerSetGet(t *testing.T) {
	t.Parallel()

	var (
		peerSet = NewPeerSet()
		peer    = newMockPeer(nil)
	)

	assert.Nil(t, peerSet.Get(peer.ID()), "expecting a nil lookup, before .Add")

	if err := peerSet.Add(peer); err != nil {
		t.Fatalf("Failed to add new peer: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		// Add them asynchronously to test the
		// concurrent guarantees of our APIs.
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			have, want := peerSet.Get(peer.ID()), peer
			assert.Equal(t, have, want, "%d: have %v, want %v", i, have, want)
		}(i)
	}
	wg.Wait()
}
