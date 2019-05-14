package p2p_test

import (
	"sync"
	"testing"

	"github.com/tendermint/tendermint/p2p"
)

// TestMockPeerBehaviour tests the MockPeerBehaviour' ability to store reported
// peer behaviour in memory indexed by the peerID
func TestMockPeerBehaviourReporter(t *testing.T) {
	var peerID p2p.ID = "MockPeer"
	pr := p2p.NewMockPeerBehaviourReporter()

	behaviours := pr.GetBehaviours(peerID)
	if len(behaviours) != 0 {
		t.Error("Expected to have no behaviours reported")
	}

	pr.Report(peerID, p2p.PeerBehaviourBadMessage)
	behaviours = pr.GetBehaviours(peerID)
	if len(behaviours) != 1 {
		t.Error("Expected the peer have one reported behaviour")
	}

	if behaviours[0] != p2p.PeerBehaviourBadMessage {
		t.Error("Expected PeerBehaviourBadMessage to have been reported")
	}
}

type scriptedBehaviours struct {
	PeerID     p2p.ID
	Behaviours []p2p.PeerBehaviour
}

type scriptItem struct {
	PeerID    p2p.ID
	Behaviour p2p.PeerBehaviour
}

func equalBehaviours(a []p2p.PeerBehaviour, b []p2p.PeerBehaviour) bool {
	if len(a) != len(b) {
		return false
	}

	var same int = 0
	for _, aBehaviour := range a {
		for _, bBehaviour := range b {
			if aBehaviour == bBehaviour {
				same++
				break
			}
		}
	}

	return same == len(a)
}

// TestPeerBehaviourConcurrency constructs a scenario in which
// multiple goroutines are using the same MockPeerBehaviourReporter instance.
// This test reproduces the conditions in which MockPeerBehaviourReporter will
// be used within a Reactor Receive method tests to ensure thread safety.
func TestMockPeerBehaviourReporterConcurrency(t *testing.T) {
	behaviourScript := []scriptedBehaviours{
		{"1", []p2p.PeerBehaviour{p2p.PeerBehaviourVote}},
		{"2", []p2p.PeerBehaviour{p2p.PeerBehaviourVote, p2p.PeerBehaviourVote, p2p.PeerBehaviourVote, p2p.PeerBehaviourVote}},
		{"3", []p2p.PeerBehaviour{p2p.PeerBehaviourBlockPart, p2p.PeerBehaviourVote, p2p.PeerBehaviourBlockPart, p2p.PeerBehaviourVote}},
		{"4", []p2p.PeerBehaviour{p2p.PeerBehaviourVote, p2p.PeerBehaviourVote, p2p.PeerBehaviourVote, p2p.PeerBehaviourVote}},
		{"5", []p2p.PeerBehaviour{p2p.PeerBehaviourBlockPart, p2p.PeerBehaviourVote, p2p.PeerBehaviourBlockPart, p2p.PeerBehaviourVote}},
	}

	var receiveWg sync.WaitGroup
	pr := p2p.NewMockPeerBehaviourReporter()
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
		reported := pr.GetBehaviours(items.PeerID)
		if !equalBehaviours(reported, items.Behaviours) {
			t.Errorf("Expected peer %s to have behaved \nExpected: %#v \nGot %#v \n",
				items.PeerID, items.Behaviours, reported)
		}
	}
}
