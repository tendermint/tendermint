package p2p

import (
	"net"
	"testing"
)

func TestStoredPeerBehaviour(t *testing.T) {
	peer := newMockPeer(net.IP{127, 0, 0, 1})
	pb := NewStoredPeerBehaviour()
	pb.Errored(peer.ID(), ErrorPeerBehaviourUnknown)

	peerErrors := pb.GetErrorBehaviours(peer.ID())
	if len(peerErrors) != 1 {
		t.Errorf("Expected the peer have one error behaviour")
	}
	if peerErrors[0] != ErrorPeerBehaviourUnknown {
		t.Errorf("Expected error to be ErrorPeerBehaviourUnknown")
	}

	pb.Behaved(peer.ID(), GoodPeerBehaviourVote)
	peerGoods := pb.GetGoodBehaviours(peer.ID())
	if len(peerGoods) != 1 {
		t.Errorf("Expected the peer have one good behaviour")
	}
	if peerGoods[0] != GoodPeerBehaviourVote {
		t.Errorf("Expected the peer to have voted")
	}
}
