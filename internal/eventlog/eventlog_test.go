package eventlog_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/google/go-cmp/cmp"

	"github.com/tendermint/tendermint/internal/eventlog"
	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	"github.com/tendermint/tendermint/types"
)

// fakeTime is a fake clock to use to control cursor assignment.
// The timeIndex method reports the current "time" and advance manually updates
// the apparent time.
type fakeTime struct{ now int64 }

func newFakeTime(init int64) *fakeTime { return &fakeTime{now: init} }

func (f *fakeTime) timeIndex() int64 { return f.now }

func (f *fakeTime) advance(d time.Duration) { f.now += int64(d) }

// eventData is a placeholder event data implementation for testing.
type eventData string

func (eventData) TypeTag() string { return "eventData" }

func TestNewError(t *testing.T) {
	lg, err := eventlog.New(eventlog.LogSettings{})
	if err == nil {
		t.Fatalf("New: got %+v, wanted error", lg)
	} else {
		t.Logf("New: got expected error: %v", err)
	}
}

func TestPruneTime(t *testing.T) {
	clk := newFakeTime(0)

	// Construct a log with a 60-second time window.
	lg, err := eventlog.New(eventlog.LogSettings{
		WindowSize: 60 * time.Second,
		Source: cursor.Source{
			TimeIndex: clk.timeIndex,
		},
	})
	if err != nil {
		t.Fatalf("New unexpectedly failed: %v", err)
	}

	// Add events up to the time window, at seconds 0, 15, 30, 45, 60.
	// None of these should be pruned (yet).
	var want []string // cursor strings
	for i := 1; i <= 5; i++ {
		want = append(want, fmt.Sprintf("%016x-%04x", clk.timeIndex(), i))
		mustAdd(t, lg, "test-event", eventData("whatever"))
		clk.advance(15 * time.Second)
	}
	// time now: 75 sec.

	// Verify that all the events we added are present.
	got := cursors(t, lg)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Cursors before pruning: (-want, +got)\n%s", diff)
	}

	// Add an event past the end of the window at second 90, and verify that
	// this triggered an age-based prune of the oldest events (0, 15) that are
	// outside the 60-second window.

	clk.advance(15 * time.Second) // time now: 90 sec.
	want = append(want[2:], fmt.Sprintf("%016x-%04x", clk.timeIndex(), 6))

	mustAdd(t, lg, "test-event", eventData("extra"))
	got = cursors(t, lg)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Cursors after pruning: (-want, +got)\n%s", diff)
	}
}

// Run a publisher and concurrent subscribers to tickle the race detector with
// concurrent add and scan operations.
func TestConcurrent(t *testing.T) {
	defer leaktest.Check(t)
	if testing.Short() {
		t.Skip("Skipping concurrency exercise because -short is set")
	}

	lg, err := eventlog.New(eventlog.LogSettings{
		WindowSize: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("New unexpectedly failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	// Publisher: Add events and handle expirations.
	wg.Add(1)
	go func() {
		defer wg.Done()

		tick := time.NewTimer(0)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-tick.C:
				_ = lg.Add("test-event", eventData(t.Format(time.RFC3339Nano)))
				tick.Reset(time.Duration(rand.Intn(50)) * time.Millisecond)
			}
		}
	}()

	// Subscribers: Wait for new events at the head of the queue.  This
	// simulates the typical operation of a subscriber by waiting for the head
	// cursor to change and then scanning down toward the unconsumed item.
	const numSubs = 16
	for i := 0; i < numSubs; i++ {
		task := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			tick := time.NewTimer(0)
			var cur cursor.Cursor
			for {
				// Simulate the subscriber being busy with other things.
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					tick.Reset(time.Duration(rand.Intn(150)) * time.Millisecond)
				}

				// Wait for new data to arrive.
				info, err := lg.WaitScan(ctx, cur, func(itm *eventlog.Item) error {
					if itm.Cursor == cur {
						return eventlog.ErrStopScan
					}
					return nil
				})
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						t.Errorf("Wait scan for task %d failed: %v", task, err)
					}
					return
				}
				cur = info.Newest
			}
		}()
	}

	time.AfterFunc(2*time.Second, cancel)
	wg.Wait()
}

func TestPruneSize(t *testing.T) {
	const maxItems = 25
	lg, err := eventlog.New(eventlog.LogSettings{
		WindowSize: 60 * time.Second,
		MaxItems:   maxItems,
	})
	if err != nil {
		t.Fatalf("New unexpectedly failed: %v", err)
	}

	// Add a lot of items to the log and verify that we never exceed the
	// specified cap.
	for i := 0; i < 60; i++ {
		mustAdd(t, lg, "test-event", eventData(strconv.Itoa(i+1)))

		if got := lg.Info().Size; got > maxItems {
			t.Errorf("After add %d: log size is %d, want ≤ %d", i+1, got, maxItems)
		}
	}
}

// mustAdd adds a single event to lg. If Add reports an error other than for
// pruning, the test fails; otherwise the error is returned.
func mustAdd(t *testing.T, lg *eventlog.Log, etype string, data types.EventData) {
	t.Helper()
	err := lg.Add(etype, data)
	if err != nil && !errors.Is(err, eventlog.ErrLogPruned) {
		t.Fatalf("Add %q failed: %v", etype, err)
	}
}

// cursors extracts the cursors from lg in ascending order of time.
func cursors(t *testing.T, lg *eventlog.Log) []string {
	t.Helper()

	var cursors []string
	if _, err := lg.Scan(func(itm *eventlog.Item) error {
		cursors = append(cursors, itm.Cursor.String())
		return nil
	}); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	reverse(cursors) // put in forward-time order for comparison
	return cursors
}

func reverse(ss []string) {
	for i, j := 0, len(ss)-1; i < j; {
		ss[i], ss[j] = ss[j], ss[i]
		i++
		j--
	}
}
