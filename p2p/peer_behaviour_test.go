package p2p

import (
	"net"
	"testing"
)

func TestStoredPeerBehaviour(t *testing.T) {
	peer := newMockPeer(net.IP{127, 0, 0, 1})
	pb := NewStoredPeerBehaviour()

	peerErrors := pb.GetErrorBehaviours(peer.ID())
	if len(peerErrors) != 0 {
		t.Errorf("Expected the peer have zero error behaviours")
	}

	pb.Errored(peer.ID(), ErrorPeerBehaviourUnknown)
	peerErrors = pb.GetErrorBehaviours(peer.ID())
	if len(peerErrors) != 1 {
		t.Errorf("Expected the peer have one error behaviour")
	}

	if peerErrors[0] != ErrorPeerBehaviourUnknown {
		t.Errorf("Expected error to be ErrorPeerBehaviourUnknown")
	}

	peerGoods := pb.GetGoodBehaviours(peer.ID())
	if len(peerGoods) != 0 {
		t.Errorf("Expected the peer have zero error behaviour")
	}

	pb.Behaved(peer.ID(), GoodPeerBehaviourVote)
	peerGoods = pb.GetGoodBehaviours(peer.ID())
	if len(peerErrors) != 1 {
		t.Errorf("Expected the peer have one good behaviour")
	}

	if peerErrors[0] != ErrorPeerBehaviourUnknown {
		t.Errorf("Expected peer to have voted")
	}
}
