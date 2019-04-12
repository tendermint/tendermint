package p2p

import (
	"net"
	"testing"
)

func TestStorePeerBehaviour(t *testing.T) {
	peer := newMockPeer(net.IP{127, 0, 0, 1})
	pb := NewStorePeerBehaviour()
	pb.Errored(peer, ErrPeerUnknown)

	peerErrors := pb.GetPeerErrors()
	if peerErrors[peer][0] != ErrPeerUnknown {
		t.Errorf("Expected to have 1 PeerError")
	}

	pb.MarkPeerAsGood(peer)
	goodPeers := pb.GetGoodPeers()
	if !goodPeers[peer] {
		t.Errorf("Expected to find the peer marked as good")
	}
}
