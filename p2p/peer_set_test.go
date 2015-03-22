package p2p

import (
	"math/rand"
	"testing"

	. "github.com/tendermint/tendermint/common"
)

// Returns an empty dummy peer
func randPeer() *Peer {
	return &Peer{
		Key: Fmt("%v.%v.%v.%v:%v", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%10000+80),
	}
}

func TestAddRemoveOne(t *testing.T) {
	peerSet := NewPeerSet()

	peer := randPeer()
	added := peerSet.Add(peer)
	if !added {
		t.Errorf("Failed to add new peer")
	}
	if peerSet.Size() != 1 {
		t.Errorf("Failed to add new peer and increment size")
	}

	peerSet.Remove(peer)
	if peerSet.Has(peer.Key) {
		t.Errorf("Failed to remove peer")
	}
	if peerSet.Size() != 0 {
		t.Errorf("Failed to remove peer and decrement size")
	}
}

func TestAddRemoveMany(t *testing.T) {
	peerSet := NewPeerSet()

	peers := []*Peer{}
	for i := 0; i < 100; i++ {
		peer := randPeer()
		added := peerSet.Add(peer)
		if !added {
			t.Errorf("Failed to add new peer")
		}
		if peerSet.Size() != i+1 {
			t.Errorf("Failed to add new peer and increment size")
		}
		peers = append(peers, peer)
	}

	for i, peer := range peers {
		peerSet.Remove(peer)
		if peerSet.Has(peer.Key) {
			t.Errorf("Failed to remove peer")
		}
		if peerSet.Size() != len(peers)-i-1 {
			t.Errorf("Failed to remove peer and decrement size")
		}
	}
}
