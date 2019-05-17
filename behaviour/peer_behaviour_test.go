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

	badMessage := bh.BadMessage(peerID, "bad message")
	pr.Report(badMessage)
	behaviours = pr.GetBehaviours(peerID)
	if len(behaviours) != 1 {
		t.Error("Expected the peer have one reported behaviour")
	}

	if behaviours[0] != badMessage {
		t.Error("Expected Bad Message to have been reported")
	}
}

type scriptedBehaviours struct {
	peerID     p2p.ID
	behaviours []bh.PeerBehaviour
}

type scriptItem struct {
	peerID    p2p.ID
	behaviour bh.PeerBehaviour
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
	var (
		peerID        p2p.ID = "MockPeer"
		consensusVote        = bh.ConsensusVote(peerID, "voted")
		blockPart            = bh.BlockPart(peerID, "blocked")
		equals               = []struct {
			left  []bh.PeerBehaviour
			right []bh.PeerBehaviour
		}{
			// Empty sets
			{[]bh.PeerBehaviour{}, []bh.PeerBehaviour{}},
			// Single behaviours
			{[]bh.PeerBehaviour{consensusVote}, []bh.PeerBehaviour{consensusVote}},
			// Equal Frequencies
			{[]bh.PeerBehaviour{consensusVote, consensusVote},
				[]bh.PeerBehaviour{consensusVote, consensusVote}},
			// Equal frequencies different orders
			{[]bh.PeerBehaviour{consensusVote, blockPart},
				[]bh.PeerBehaviour{blockPart, consensusVote}},
		}
		unequals = []struct {
			left  []bh.PeerBehaviour
			right []bh.PeerBehaviour
		}{
			// Comparing empty sets to non empty sets
			{[]bh.PeerBehaviour{}, []bh.PeerBehaviour{consensusVote}},
			// Different behaviours
			{[]bh.PeerBehaviour{consensusVote}, []bh.PeerBehaviour{blockPart}},
			// Same behaviour with different frequencies
			{[]bh.PeerBehaviour{consensusVote},
				[]bh.PeerBehaviour{consensusVote, consensusVote}},
		}
	)

	for _, test := range equals {
		if !equalBehaviours(test.left, test.right) {
			t.Errorf("Expected %#v and %#v to be equal", test.left, test.right)
		}
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
	var (
		peerID          p2p.ID = "MockPeer"
		consensusVote          = bh.ConsensusVote(peerID, "voted")
		blockPart              = bh.BlockPart(peerID, "blocked")
		behaviourScript        = []scriptedBehaviours{
			{"1", []bh.PeerBehaviour{consensusVote}},
			{"2", []bh.PeerBehaviour{consensusVote, consensusVote, consensusVote, consensusVote}},
			{"3", []bh.PeerBehaviour{blockPart, consensusVote, blockPart, consensusVote}},
			{"4", []bh.PeerBehaviour{consensusVote, consensusVote, consensusVote, consensusVote}},
			{"5", []bh.PeerBehaviour{blockPart, consensusVote, blockPart, consensusVote}},
		}
	)

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
					pr.Report(pb.behaviour)
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
			for _, reason := range item.behaviours {
				scriptItems <- scriptItem{item.peerID, reason}
			}
		}
	}()

	sendingWg.Wait()

	for i := 0; i < numConsumers; i++ {
		done <- 1
	}

	receiveWg.Wait()

	for _, items := range behaviourScript {
		reported := pr.GetBehaviours(items.peerID)
		if !equalBehaviours(reported, items.behaviours) {
			t.Errorf("Expected peer %s to have behaved \nExpected: %#v \nGot %#v \n",
				items.peerID, items.behaviours, reported)
		}
	}
}
