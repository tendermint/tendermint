package queue

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		desc string
		opts Options
		want error
	}{
		{"empty options", Options{}, errHardLimit},
		{"zero limit negative quota", Options{SoftQuota: -1}, errHardLimit},
		{"zero limit and quota", Options{SoftQuota: 0}, errHardLimit},
		{"zero limit", Options{SoftQuota: 1, HardLimit: 0}, errHardLimit},
		{"limit less than quota", Options{SoftQuota: 5, HardLimit: 3}, errHardLimit},
		{"negative credit", Options{SoftQuota: 1, HardLimit: 1, BurstCredit: -6}, errBurstCredit},
		{"valid default credit", Options{SoftQuota: 1, HardLimit: 2, BurstCredit: 0}, nil},
		{"valid explicit credit", Options{SoftQuota: 1, HardLimit: 5, BurstCredit: 10}, nil},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := New(test.opts)
			if err != test.want {
				t.Errorf("New(%+v): got (%+v, %v), want err=%v", test.opts, got, err, test.want)
			}
		})
	}
}

type testQueue struct {
	t *testing.T
	*Queue
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

func mustQueue(t *testing.T, opts Options) testQueue {
	t.Helper()

	q, err := New(opts)
	if err != nil {
		t.Fatalf("New(%+v): unexpected error: %v", opts, err)
	}
	return testQueue{t: t, Queue: q}
}

func TestHardLimit(t *testing.T) {
	q := mustQueue(t, Options{SoftQuota: 1, HardLimit: 1})
	q.mustAdd("foo")
	if err := q.Add("bar"); err != ErrQueueFull {
		t.Errorf("Add: got err=%v, want %v", err, ErrQueueFull)
	}
}

func TestSoftQuota(t *testing.T) {
	q := mustQueue(t, Options{SoftQuota: 1, HardLimit: 4})
	q.mustAdd("foo")
	q.mustAdd("bar")
	if err := q.Add("baz"); err != ErrNoCredit {
		t.Errorf("Add: got err=%v, want %v", err, ErrNoCredit)
	}
}

func TestBurstCredit(t *testing.T) {
	q := mustQueue(t, Options{SoftQuota: 2, HardLimit: 5})
	q.mustAdd("foo")
	q.mustAdd("bar")

	// We should still have all our initial credit.
	if q.credit < 2 {
		t.Errorf("Wrong credit: got %f, want ≥ 2", q.credit)
	}

	// Removing an item below soft quota should increase our credit.
	q.mustRemove("foo")
	if q.credit <= 2 {
		t.Errorf("wrong credit: got %f, want > 2", q.credit)
	}

	// Credit should be capped by the hard limit.
	q.mustRemove("bar")
	q.mustAdd("baz")
	q.mustRemove("baz")
	if cap := float64(q.hardLimit - q.softQuota); q.credit > cap {
		t.Errorf("Wrong credit: got %f, want ≤ %f", q.credit, cap)
	}
}

func TestClose(t *testing.T) {
	q := mustQueue(t, Options{SoftQuota: 2, HardLimit: 10})
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := mustQueue(t, Options{SoftQuota: 2, HardLimit: 2})

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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		const input = "figgy pudding"
		q.mustAdd(input)

		got, err := q.Wait(ctx)
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
			got, err := q.Wait(ctx)
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
			got, err := q.Wait(ctx)
			if err != ErrQueueClosed {
				t.Errorf("Wait: got (%v, %v), want %v", got, err, ErrQueueClosed)
			}
		}()

		q.Close()
		<-done
	})
}
