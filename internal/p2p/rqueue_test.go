package p2p

import (
	"context"
	"testing"
	"time"
)

func TestSimpleQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up a small queue with very small buffers so we can
	// watch it shed load, then send a bunch of messages to the
	// queue, most of which we'll watch it drop.
	sq := newSimplePriorityQueue(ctx, 1, nil)
	for i := 0; i < 100; i++ {
		sq.enqueue() <- Envelope{From: "merlin"}
	}

	seen := 0

RETRY:
	for seen <= 2 {
		select {
		case e := <-sq.dequeue():
			if e.From != "merlin" {
				continue
			}
			seen++
		case <-time.After(10 * time.Millisecond):
			break RETRY
		}
	}
	// if we don't see any messages, then it's just broken.
	if seen == 0 {
		t.Errorf("seen %d messages, should have seen more than one", seen)
	}
	// ensure that load shedding happens: there can be at most 3
	// messages that we get out of this, one that was buffered
	// plus 2 that were under the cap, everything else gets
	// dropped.
	if seen > 3 {
		t.Errorf("saw %d messages, should have seen 5 or fewer", seen)
	}

}
