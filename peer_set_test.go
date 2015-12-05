package p2p

import (
	"math/rand"
	"testing"

	. "github.com/tendermint/go-common"
)

// Returns an empty dummy peer
func randPeer() *Peer {
	return &Peer{
		Key: RandStr(12),
		NodeInfo: &NodeInfo{
			RemoteAddr: Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
			ListenAddr: Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
		},
	}
}

func TestAddRemoveOne(t *testing.T) {
	peerSet := NewPeerSet()

	peer := randPeer()
	err := peerSet.Add(peer)
	if err != nil {
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
	N := 100
	for i := 0; i < N; i++ {
		peer := randPeer()
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
		if peerSet.Has(peer.Key) {
			t.Errorf("Failed to remove peer")
		}
		if peerSet.Size() != len(peers)-i-1 {
			t.Errorf("Failed to remove peer and decrement size")
		}
	}
}
