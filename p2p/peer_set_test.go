package p2p

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

// Returns an empty dummy peer
func randPeer() *Peer {
	return &Peer{
		Key: uuid.New(),
		NodeInfo: &types.NodeInfo{
			Host: Fmt("%v.%v.%v.%v", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
		},
	}
}

func TestAddRemoveOne(t *testing.T) {
	peerSet := NewPeerSet()

	peer := randPeer()
	added := peerSet.Add(peer)
	if err := added; err != nil {
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
	maxPeersPerIPRange = [4]int{N, N, N, N}
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

func newPeerInIPRange(ipBytes ...string) *Peer {
	ips := make([]string, 4)
	for i, ipByte := range ipBytes {
		ips[i] = ipByte
	}

	for i := len(ipBytes); i < 4; i++ {
		ips[i] = Fmt("%v", rand.Int()%256)
	}

	ipS := strings.Join(ips, ".")
	return &Peer{
		Key: uuid.New(),
		NodeInfo: &types.NodeInfo{
			Host: ipS,
		},
	}
}

func TestIPRanges(t *testing.T) {
	peerSet := NewPeerSet()

	// test  /8
	maxPeersPerIPRange = [4]int{2, 2, 2, 2}
	peer := newPeerInIPRange("54")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54")
	if err := peerSet.Add(peer); err == nil {
		t.Errorf("Added peer when we shouldn't have")
	}
	peer = newPeerInIPRange("55")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}

	// test  /16
	peerSet = NewPeerSet()
	maxPeersPerIPRange = [4]int{3, 2, 1, 1}
	peer = newPeerInIPRange("54", "112")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54", "112")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54", "112")
	if err := peerSet.Add(peer); err == nil {
		t.Errorf("Added peer when we shouldn't have")
	}
	peer = newPeerInIPRange("54", "113")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}

	// test  /24
	peerSet = NewPeerSet()
	maxPeersPerIPRange = [4]int{5, 3, 2, 1}
	peer = newPeerInIPRange("54", "112", "11")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54", "112", "11")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54", "112", "11")
	if err := peerSet.Add(peer); err == nil {
		t.Errorf("Added peer when we shouldn't have")
	}
	peer = newPeerInIPRange("54", "112", "12")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}

	// test  /32
	peerSet = NewPeerSet()
	maxPeersPerIPRange = [4]int{11, 7, 5, 2}
	peer = newPeerInIPRange("54", "112", "11", "10")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54", "112", "11", "10")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
	peer = newPeerInIPRange("54", "112", "11", "10")
	if err := peerSet.Add(peer); err == nil {
		t.Errorf("Added peer when we shouldn't have")
	}
	peer = newPeerInIPRange("54", "112", "11", "11")
	if err := peerSet.Add(peer); err != nil {
		t.Errorf("Failed to add new peer")
	}
}
