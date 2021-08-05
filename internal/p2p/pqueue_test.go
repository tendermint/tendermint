package p2p

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/libs/log"
)

func TestCloseWhileDequeueFull(t *testing.T) {
	enqueueLength := 5
	chDescs := []ChannelDescriptor{
		{ID: 0x01, Priority: 1, MaxSendBytes: 4},
	}
	pqueue := newPQScheduler(log.NewNopLogger(), NopMetrics(), chDescs, uint(enqueueLength), 1, 120)

	for i := 0; i < enqueueLength; i++ {
		pqueue.enqueue() <- Envelope{
			channelID: 0x01,
			Message:   &testMessage{Value: "foo"}, // 5 bytes
		}
	}

	go pqueue.process()

	// sleep to allow context switch for process() to run
	time.Sleep(10 * time.Millisecond)
	doneCh := make(chan struct{})
	go func() {
		pqueue.close()
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("pqueue failed to close")
	}
}
