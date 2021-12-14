package nqueue_test

import (
	"context"
	"testing"
	"time"

	"github.com/tendermint/tendermint/internal/libs/nqueue"
)

type testQueue struct {
	t *testing.T
	*nqueue.Queue
}

func (q testQueue) mustAdd(item string) {
	q.t.Helper()
	if err := q.Add(item); err != nil {
		q.t.Errorf("Add(%q): unexpected error: %v", item, err)
	}
}

func (q testQueue) mustRemove(want string) {
	q.t.Helper()
	got, ok := q.Remove()
	if !ok {
		q.t.Error("Remove: queue is empty")
	} else if got.(string) != want {
		q.t.Errorf("Remove: got %q, want %q", got, want)
	}
}

func newTestQueue(t *testing.T) testQueue {
	return testQueue{t: t, Queue: nqueue.New()}
}

func TestClose(t *testing.T) {
	q := newTestQueue(t)
	q.mustAdd("alpha")
	q.mustAdd("bravo")
	q.mustAdd("charlie")
	q.Close()

	// After closing the queue, subsequent writes should fail.
	if err := q.Add("foxtrot"); err == nil {
		t.Error("Add should have failed after Close")
	}

	// However, the remaining contents of the queue should still work.
	q.mustRemove("alpha")
	q.mustRemove("bravo")
	q.mustRemove("charlie")
}

func TestWait(t *testing.T) {
	q := newTestQueue(t)

	// A wait on an empty queue should time out.
	t.Run("WaitTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		got, err := q.Wait(ctx)
		if err == nil {
			t.Errorf("Wait: got %v, want error", got)
		} else {
			t.Logf("Wait correctly failed: %v", err)
		}
	})

	// A wait on a non-empty queue should report an item.
	t.Run("WaitNonEmpty", func(t *testing.T) {
		const input = "figgy pudding"
		q.mustAdd(input)

		got, err := q.Wait(context.Background())
		if err != nil {
			t.Errorf("Wait: unexpected error: %v", err)
		} else if got != input {
			t.Errorf("Wait: got %q, want %q", got, input)
		}
	})

	// Wait should block until an item arrives.
	t.Run("WaitOnEmpty", func(t *testing.T) {
		const input = "fleet footed kittens"

		done := make(chan struct{})
		go func() {
			defer close(done)
			got, err := q.Wait(context.Background())
			if err != nil {
				t.Errorf("Wait: unexpected error: %v", err)
			} else if got != input {
				t.Errorf("Wait: got %q, want %q", got, input)
			}
		}()

		q.mustAdd(input)
		<-done
	})

	// Closing the queue unblocks a wait.
	t.Run("UnblockOnClose", func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			got, err := q.Wait(context.Background())
			if err != nqueue.ErrQueueClosed {
				t.Errorf("Wait: got (%v, %v), want %v", got, err, nqueue.ErrQueueClosed)
			}
		}()

		q.Close()
		<-done
	})
}
