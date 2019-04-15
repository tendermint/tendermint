package p2p

import (
	"net"
	"testing"
)

func TestStoredPeerBehaviour(t *testing.T) {
	peer := newMockPeer(net.IP{127, 0, 0, 1})
	pb := NewStoredPeerBehaviour()
	pb.Errored(peer, ErrorPeerBehaviourUnknown)

	peerErrors := pb.GetErrored()
	if peerErrors[peer][0] != ErrorPeerBehaviourUnknown {
		t.Errorf("Expected the peer to have errored")
	}

	pb.Behaved(peer, GoodPeerBehaviourVote)
	goodPeers := pb.GetBehaved()
	if goodPeers[peer][0] != GoodPeerBehaviourVote {
		t.Errorf("Expected the peer to have voted")
	}
}
