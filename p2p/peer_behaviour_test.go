package p2p

import (
	"net"
	"sync"
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
	if len(peerGoods) != 1 {
		t.Errorf("Expected the peer have one good behaviour")
	}

	if peerGoods[0] != GoodPeerBehaviourVote {
		t.Errorf("Expected peer to have voted")
	}

}

type scriptedGoodBehaviour struct {
	PeerID     ID
	Behaviours []GoodPeerBehaviour
}

type scriptedErrorBehaviour struct {
	PeerID     ID
	Behaviours []ErrorPeerBehaviour
}

type goodScriptItem struct {
	PeerID    ID
	Behaviour GoodPeerBehaviour
}

type errorScriptItem struct {
	PeerID    ID
	Behaviour ErrorPeerBehaviour
}

func equalGoodBehaviours(a []GoodPeerBehaviour, b []GoodPeerBehaviour) bool {
	if len(a) != len(b) {
		return false
	}
	same := make([]GoodPeerBehaviour, len(a))

	for i, aBehaviour := range a {
		for _, bBehaviour := range b {
			if aBehaviour == bBehaviour {
				same[i] = aBehaviour
			}
		}
	}

	return len(same) == len(a)
}

func equalErrorBehaviours(a []ErrorPeerBehaviour, b []ErrorPeerBehaviour) bool {
	if len(a) != len(b) {
		return false
	}
	same := make([]ErrorPeerBehaviour, len(a))

	for i, aBehaviour := range a {
		for _, bBehaviour := range b {
			if aBehaviour == bBehaviour {
				same[i] = aBehaviour
			}
		}
	}

	return len(same) == len(a)
}

// TestStoredPeerBehaviourConcurrency constructs a scenario in which
// multiple goroutines are using the same StoredPeerBehaviour instance.
// This test is meant to reproduce the conditions in which StoredPeerBehaviour
// will be used within a Reactor Receive method test and ensure thread safety.
func TestStoredPeerBehaviourConcurrency(t *testing.T) {
	goodBehaviourScript := []scriptedGoodBehaviour{
		{"1", []GoodPeerBehaviour{GoodPeerBehaviourVote}},
		{"2", []GoodPeerBehaviour{GoodPeerBehaviourVote, GoodPeerBehaviourVote, GoodPeerBehaviourVote, GoodPeerBehaviourVote}},
		{"3", []GoodPeerBehaviour{GoodPeerBehaviourBlockPart, GoodPeerBehaviourVote, GoodPeerBehaviourBlockPart, GoodPeerBehaviourVote}},
		{"4", []GoodPeerBehaviour{GoodPeerBehaviourVote, GoodPeerBehaviourVote, GoodPeerBehaviourVote, GoodPeerBehaviourVote}},
		{"5", []GoodPeerBehaviour{GoodPeerBehaviourBlockPart, GoodPeerBehaviourVote, GoodPeerBehaviourBlockPart, GoodPeerBehaviourVote}},
	}

	errorBehaviourScript := []scriptedErrorBehaviour{
		{"1", []ErrorPeerBehaviour{ErrorPeerBehaviourUnknown}},
		{"2", []ErrorPeerBehaviour{ErrorPeerBehaviourUnknown, ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourUnknown}},
		{"3", []ErrorPeerBehaviour{ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourUnknown, ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourUnknown}},
		{"4", []ErrorPeerBehaviour{ErrorPeerBehaviourUnknown, ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourUnknown}},
		{"5", []ErrorPeerBehaviour{ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourUnknown, ErrorPeerBehaviourBadMessage, ErrorPeerBehaviourUnknown}},
	}

	var receiveWg sync.WaitGroup
	pb := NewStoredPeerBehaviour()
	goodScriptItems := make(chan goodScriptItem)
	errorScriptItems := make(chan errorScriptItem)
	done := make(chan int)
	numConsumers := 3
	for i := 0; i < numConsumers; i++ {
		receiveWg.Add(1)
		go func() {
			defer receiveWg.Done()
			for {
				select {
				case gb := <-goodScriptItems:
					pb.Behaved(gb.PeerID, gb.Behaviour)
				case eb := <-errorScriptItems:
					pb.Errored(eb.PeerID, eb.Behaviour)
				case <-done:
					return
				}
			}
		}()
	}

	var sendingWg sync.WaitGroup
	sendingWg.Add(1)
	go func() {
		defer sendingWg.Done()
		for _, item := range goodBehaviourScript {
			for _, reason := range item.Behaviours {
				goodScriptItems <- goodScriptItem{item.PeerID, reason}
			}
		}
	}()

	sendingWg.Add(1)
	go func() {
		defer sendingWg.Done()
		for _, item := range errorBehaviourScript {
			for _, reason := range item.Behaviours {
				errorScriptItems <- errorScriptItem{item.PeerID, reason}
			}
		}
	}()

	sendingWg.Wait()

	for i := 0; i < numConsumers; i++ {
		done <- 1
	}

	receiveWg.Wait()

	for _, items := range goodBehaviourScript {
		if !equalGoodBehaviours(pb.GetGoodBehaviours(items.PeerID), items.Behaviours) {
			t.Errorf("Expected peer %s to have behaved \n", items.PeerID)
		}
	}

	for _, items := range errorBehaviourScript {
		if !equalErrorBehaviours(pb.GetErrorBehaviours(items.PeerID), items.Behaviours) {
			t.Errorf("Expected peer %s to have errored \n", items.PeerID)
		}
	}
}
