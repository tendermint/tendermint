package behavior_test

import (
	"sync"
	"testing"

	bh "github.com/tendermint/tendermint/behavior"
	"github.com/tendermint/tendermint/p2p"
)

// TestMockReporter tests the MockReporter's ability to store reported
// peer behavior in memory indexed by the peerID.
func TestMockReporter(t *testing.T) {
	var peerID p2p.ID = "MockPeer"
	pr := bh.NewMockReporter()

	behaviors := pr.GetBehaviours(peerID)
	if len(behaviors) != 0 {
		t.Error("Expected to have no behaviors reported")
	}

	badMessage := bh.BadMessage(peerID, "bad message")
	if err := pr.Report(badMessage); err != nil {
		t.Error(err)
	}
	behaviors = pr.GetBehaviours(peerID)
	if len(behaviors) != 1 {
		t.Error("Expected the peer have one reported behavior")
	}

	if behaviors[0] != badMessage {
		t.Error("Expected Bad Message to have been reported")
	}
}

type scriptItem struct {
	peerID   p2p.ID
	behavior bh.PeerBehavior
}

// equalBehaviours returns true if a and b contain the same PeerBehaviors with
// the same freequencies and otherwise false.
func equalBehaviours(a []bh.PeerBehavior, b []bh.PeerBehavior) bool {
	aHistogram := map[bh.PeerBehavior]int{}
	bHistogram := map[bh.PeerBehavior]int{}

	for _, behavior := range a {
		aHistogram[behavior]++
	}

	for _, behavior := range b {
		bHistogram[behavior]++
	}

	if len(aHistogram) != len(bHistogram) {
		return false
	}

	for _, behavior := range a {
		if aHistogram[behavior] != bHistogram[behavior] {
			return false
		}
	}

	for _, behavior := range b {
		if bHistogram[behavior] != aHistogram[behavior] {
			return false
		}
	}

	return true
}

// TestEqualPeerBehaviors tests that equalBehaviours can tell that two slices
// of peer behaviors can be compared for the behaviors they contain and the
// freequencies that those behaviors occur.
func TestEqualPeerBehaviors(t *testing.T) {
	var (
		peerID        p2p.ID = "MockPeer"
		consensusVote        = bh.ConsensusVote(peerID, "voted")
		blockPart            = bh.BlockPart(peerID, "blocked")
		equals               = []struct {
			left  []bh.PeerBehavior
			right []bh.PeerBehavior
		}{
			// Empty sets
			{[]bh.PeerBehavior{}, []bh.PeerBehavior{}},
			// Single behaviors
			{[]bh.PeerBehavior{consensusVote}, []bh.PeerBehavior{consensusVote}},
			// Equal Frequencies
			{[]bh.PeerBehavior{consensusVote, consensusVote},
				[]bh.PeerBehavior{consensusVote, consensusVote}},
			// Equal frequencies different orders
			{[]bh.PeerBehavior{consensusVote, blockPart},
				[]bh.PeerBehavior{blockPart, consensusVote}},
		}
		unequals = []struct {
			left  []bh.PeerBehavior
			right []bh.PeerBehavior
		}{
			// Comparing empty sets to non empty sets
			{[]bh.PeerBehavior{}, []bh.PeerBehavior{consensusVote}},
			// Different behaviors
			{[]bh.PeerBehavior{consensusVote}, []bh.PeerBehavior{blockPart}},
			// Same behavior with different frequencies
			{[]bh.PeerBehavior{consensusVote},
				[]bh.PeerBehavior{consensusVote, consensusVote}},
		}
	)

	for _, test := range equals {
		if !equalBehaviours(test.left, test.right) {
			t.Errorf("expected %#v and %#v to be equal", test.left, test.right)
		}
	}

	for _, test := range unequals {
		if equalBehaviours(test.left, test.right) {
			t.Errorf("expected %#v and %#v to be unequal", test.left, test.right)
		}
	}
}

// TestPeerBehaviorConcurrency constructs a scenario in which
// multiple goroutines are using the same MockReporter instance.
// This test reproduces the conditions in which MockReporter will
// be used within a Reactor `Receive` method tests to ensure thread safety.
func TestMockPeerBehaviorReporterConcurrency(t *testing.T) {
	var (
		behaviorScript = []struct {
			peerID    p2p.ID
			behaviors []bh.PeerBehavior
		}{
			{"1", []bh.PeerBehavior{bh.ConsensusVote("1", "")}},
			{"2", []bh.PeerBehavior{bh.ConsensusVote("2", ""), bh.ConsensusVote("2", ""), bh.ConsensusVote("2", "")}},
			{
				"3",
				[]bh.PeerBehavior{bh.BlockPart("3", ""),
					bh.ConsensusVote("3", ""),
					bh.BlockPart("3", ""),
					bh.ConsensusVote("3", "")}},
			{
				"4",
				[]bh.PeerBehavior{bh.ConsensusVote("4", ""),
					bh.ConsensusVote("4", ""),
					bh.ConsensusVote("4", ""),
					bh.ConsensusVote("4", "")}},
			{
				"5",
				[]bh.PeerBehavior{bh.BlockPart("5", ""),
					bh.ConsensusVote("5", ""),
					bh.BlockPart("5", ""),
					bh.ConsensusVote("5", "")}},
		}
	)

	var receiveWg sync.WaitGroup
	pr := bh.NewMockReporter()
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
					if err := pr.Report(pb.behavior); err != nil {
						t.Error(err)
					}
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
		for _, item := range behaviorScript {
			for _, reason := range item.behaviors {
				scriptItems <- scriptItem{item.peerID, reason}
			}
		}
	}()

	sendingWg.Wait()

	for i := 0; i < numConsumers; i++ {
		done <- 1
	}

	receiveWg.Wait()

	for _, items := range behaviorScript {
		reported := pr.GetBehaviours(items.peerID)
		if !equalBehaviours(reported, items.behaviors) {
			t.Errorf("expected peer %s to have behaved \nExpected: %#v \nGot %#v \n",
				items.peerID, items.behaviors, reported)
		}
	}
}
