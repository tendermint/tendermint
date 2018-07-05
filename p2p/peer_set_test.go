package p2p

import (
	"math/rand"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	crypto "github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
)

// Returns an empty kvstore peer
func randPeer(ip net.IP) *peer {
	if ip == nil {
		ip = net.IP{127, 0, 0, 1}
	}

	nodeKey := NodeKey{PrivKey: crypto.GenPrivKeyEd25519()}
	p := &peer{
		nodeInfo: NodeInfo{
			ID:         nodeKey.ID(),
			ListenAddr: cmn.Fmt("%v.%v.%v.%v:26656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
		},
	}

	p.ip = ip

	return p
}

func TestPeerSetAddRemoveOne(t *testing.T) {
	t.Parallel()

	peerSet := NewPeerSet()

	var peerList []Peer
	for i := 0; i < 5; i++ {
		p := randPeer(net.IP{127, 0, 0, byte(i)})
		if err := peerSet.Add(p); err != nil {
			t.Error(err)
		}
		peerList = append(peerList, p)
	}

	n := len(peerList)
	// 1. Test removing from the front
	for i, peerAtFront := range peerList {
		peerSet.Remove(peerAtFront)
		wantSize := n - i - 1
		for j := 0; j < 2; j++ {
			assert.Equal(t, false, peerSet.Has(peerAtFront.ID()), "#%d Run #%d: failed to remove peer", i, j)
			assert.Equal(t, wantSize, peerSet.Size(), "#%d Run #%d: failed to remove peer and decrement size", i, j)
			// Test the route of removing the now non-existent element
			peerSet.Remove(peerAtFront)
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
		peerSet.Remove(peerAtEnd)
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
		peer := randPeer(net.IP{127, 0, 0, byte(i)})
		if err := peerSet.Add(peer); err != nil {
			t.Errorf("Failed to add new peer")
		}
		if peerSet.Size() != i+1 {
			t.Errorf("Failed to add new peer and increment size")
		}
		peers = append(peers, peer)
	}

	for i, peer := range peers {
		peerSet.Remove(peer)
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
	peer := randPeer(nil)

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
		peer    = randPeer(nil)
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
