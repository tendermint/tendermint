package behaviour_test

import (
	"sync"
	"testing"

	bh "github.com/tendermint/tendermint/behaviour"
	"github.com/tendermint/tendermint/p2p"
)

// TestMockPeerBehaviour tests the MockPeerBehaviour' ability to store reported
// peer behaviour in memory indexed by the peerID
func TestMockPeerBehaviourReporter(t *testing.T) {
	var peerID p2p.ID = "MockPeer"
	pr := bh.NewMockPeerBehaviourReporter()

	behaviours := pr.GetBehaviours(peerID)
	if len(behaviours) != 0 {
		t.Error("Expected to have no behaviours reported")
	}

	pr.Report(peerID, bh.PeerBehaviourBadMessage)
	behaviours = pr.GetBehaviours(peerID)
	if len(behaviours) != 1 {
		t.Error("Expected the peer have one reported behaviour")
	}

	if behaviours[0] != bh.PeerBehaviourBadMessage {
		t.Error("Expected PeerBehaviourBadMessage to have been reported")
	}
}

type scriptedBehaviours struct {
	PeerID     p2p.ID
	Behaviours []bh.PeerBehaviour
}

type scriptItem struct {
	PeerID    p2p.ID
	Behaviour bh.PeerBehaviour
}

// equalBehaviours returns true if a and b contain the same PeerBehaviours with
// the same freequency and otherwise false.
func equalBehaviours(a []bh.PeerBehaviour, b []bh.PeerBehaviour) bool {
	aHistogram := map[bh.PeerBehaviour]int{}
	bHistogram := map[bh.PeerBehaviour]int{}

	for _, behaviour := range a {
		aHistogram[behaviour] += 1
	}

	for _, behaviour := range b {
		bHistogram[behaviour] += 1
	}

	if len(aHistogram) != len(bHistogram) {
		return false
	}

	for _, behaviour := range a {
		if aHistogram[behaviour] != bHistogram[behaviour] {
			return false
		}
	}

	for _, behaviour := range b {
		if bHistogram[behaviour] != aHistogram[behaviour] {
			return false
		}
	}

	return true
}

// TestEqualPeerBehaviours tests that equalBehaviours can tell that two slices
// of peer behaviours can be compared for the behaviours they contain and the
// freequencies that those behaviours occur.
func TestEqualPeerBehaviours(t *testing.T) {
	equals := []struct {
		left  []bh.PeerBehaviour
		right []bh.PeerBehaviour
	}{
		// Empty sets
		{[]bh.PeerBehaviour{}, []bh.PeerBehaviour{}},
		// Single behaviours
		{[]bh.PeerBehaviour{bh.PeerBehaviourVote}, []bh.PeerBehaviour{bh.PeerBehaviourVote}},
		// Equal Frequencies
		{[]bh.PeerBehaviour{bh.PeerBehaviourVote, bh.PeerBehaviourVote},
			[]bh.PeerBehaviour{bh.PeerBehaviourVote, bh.PeerBehaviourVote}},
		// Equal frequencies different orders
		{[]bh.PeerBehaviour{bh.PeerBehaviourVote, bh.PeerBehaviourBlockPart},
			[]bh.PeerBehaviour{bh.PeerBehaviourBlockPart, bh.PeerBehaviourVote}},
	}

	for _, test := range equals {
		if !equalBehaviours(test.left, test.right) {
			t.Errorf("Expected %#v and %#v to be equal", test.left, test.right)
		}
	}

	unequals := []struct {
		left  []bh.PeerBehaviour
		right []bh.PeerBehaviour
	}{
		// Comparing empty sets to non empty sets
		{[]bh.PeerBehaviour{}, []bh.PeerBehaviour{bh.PeerBehaviourVote}},
		// Different behaviours
		{[]bh.PeerBehaviour{bh.PeerBehaviourVote}, []bh.PeerBehaviour{bh.PeerBehaviourBlockPart}},
		// Same behaviour with different frequencies
		{[]bh.PeerBehaviour{bh.PeerBehaviourVote},
			[]bh.PeerBehaviour{bh.PeerBehaviourVote, bh.PeerBehaviourVote}},
	}

	for _, test := range unequals {
		if equalBehaviours(test.left, test.right) {
			t.Errorf("Expected %#v and %#v to be unequal", test.left, test.right)
		}
	}
}

// TestPeerBehaviourConcurrency constructs a scenario in which
// multiple goroutines are using the same MockPeerBehaviourReporter instance.
// This test reproduces the conditions in which MockPeerBehaviourReporter will
// be used within a Reactor Receive method tests to ensure thread safety.
func TestMockPeerBehaviourReporterConcurrency(t *testing.T) {
	behaviourScript := []scriptedBehaviours{
		{"1", []bh.PeerBehaviour{bh.PeerBehaviourVote}},
		{"2", []bh.PeerBehaviour{bh.PeerBehaviourVote, bh.PeerBehaviourVote, bh.PeerBehaviourVote, bh.PeerBehaviourVote}},
		{"3", []bh.PeerBehaviour{bh.PeerBehaviourBlockPart, bh.PeerBehaviourVote, bh.PeerBehaviourBlockPart, bh.PeerBehaviourVote}},
		{"4", []bh.PeerBehaviour{bh.PeerBehaviourVote, bh.PeerBehaviourVote, bh.PeerBehaviourVote, bh.PeerBehaviourVote}},
		{"5", []bh.PeerBehaviour{bh.PeerBehaviourBlockPart, bh.PeerBehaviourVote, bh.PeerBehaviourBlockPart, bh.PeerBehaviourVote}},
	}

	var receiveWg sync.WaitGroup
	pr := bh.NewMockPeerBehaviourReporter()
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
