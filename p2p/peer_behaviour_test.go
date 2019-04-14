package p2p

import (
	"net"
	"testing"
)

func TestStorePeerBehaviour(t *testing.T) {
	peer := newMockPeer(net.IP{127, 0, 0, 1})
	pb := NewStorePeerBehaviour()
	pb.Errored(peer, ErrorBehaviourUnknown)

	peerErrors := pb.GetErrored()
	if peerErrors[peer][0] != ErrorBehaviourUnknown {
		t.Errorf("Expected the peer to have errored")
	}

	pb.Behaved(peer, GoodBehaviourVote)
	goodPeers := pb.GetBehaved()
	if goodPeers[peer][0] != GoodBehaviourVote {
		t.Errorf("Expected the peer to have voted")
	}
}
