package p2p

import (
	"net"
	"sync"
	"testing"
)

// TestMockPeerBehaviour tests the MockPeerBehaviour' ability to store reported
// peer behaviour in memory indexed by the peerID
func TestMockPeerBehaviourReporter(t *testing.T) {
	peer := newMockPeer(net.IP{127, 0, 0, 1})
	pr := NewMockPeerBehaviourReporter()

	behaviours := pr.GetBehaviours(peer.ID())
	if len(behaviours) != 0 {
		t.Errorf("Expected to have no behaviours reported")
	}

	pr.Report(peer.ID(), PeerBehaviourBadMessage)
	behaviours = pr.GetBehaviours(peer.ID())
	if len(behaviours) != 1 {
		t.Errorf("Expected the peer have one reported behaviour")
	}

	if behaviours[0] != PeerBehaviourBadMessage {
		t.Errorf("Expected PeerBehaviourBadMessage to have been reported")
	}
}

type scriptedBehaviours struct {
	PeerID     ID
	Behaviours []PeerBehaviour
}

type scriptItem struct {
	PeerID    ID
	Behaviour PeerBehaviour
}

func equalBehaviours(a []PeerBehaviour, b []PeerBehaviour) bool {
	if len(a) != len(b) {
		return false
	}
	same := make([]PeerBehaviour, len(a))

	for i, aBehaviour := range a {
		for _, bBehaviour := range b {
			if aBehaviour == bBehaviour {
				same[i] = aBehaviour
			}
		}
	}

	return len(same) == len(a)
}

// TestPeerBehaviourConcurrency constructs a scenario in which
// multiple goroutines are using the same MockPeerBehaviourReporter instance.
// This test reproduces the conditions in which MockPeerBehaviourReporter will
// be used within a Reactor Receive method tests to ensure thread safety.
func TestMockPeerBehaviourReporterConcurrency(t *testing.T) {
	behaviourScript := []scriptedBehaviours{
		{"1", []PeerBehaviour{PeerBehaviourVote}},
		{"2", []PeerBehaviour{PeerBehaviourVote, PeerBehaviourVote, PeerBehaviourVote, PeerBehaviourVote}},
		{"3", []PeerBehaviour{PeerBehaviourBlockPart, PeerBehaviourVote, PeerBehaviourBlockPart, PeerBehaviourVote}},
		{"4", []PeerBehaviour{PeerBehaviourVote, PeerBehaviourVote, PeerBehaviourVote, PeerBehaviourVote}},
		{"5", []PeerBehaviour{PeerBehaviourBlockPart, PeerBehaviourVote, PeerBehaviourBlockPart, PeerBehaviourVote}},
	}

	var receiveWg sync.WaitGroup
	pr := NewMockPeerBehaviourReporter()
	scriptItems := make(chan scriptItem)
	done := make(chan int)
	numConsumers := 3
	for i := 0; i < numConsumers; i++ {
		receiveWg.Add(1)
		go func() {
			defer receiveWg.Done()
			for {
				select {
				case pb := <-scriptItems:
					pr.Report(pb.PeerID, pb.Behaviour)
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
		for _, item := range behaviourScript {
			for _, reason := range item.Behaviours {
				scriptItems <- scriptItem{item.PeerID, reason}
			}
		}
	}()

	sendingWg.Wait()

	for i := 0; i < numConsumers; i++ {
		done <- 1
	}

	receiveWg.Wait()

	for _, items := range behaviourScript {
		if !equalBehaviours(pr.GetBehaviours(items.PeerID), items.Behaviours) {
			t.Errorf("Expected peer %s to have behaved \n", items.PeerID)
		}
	}
}
