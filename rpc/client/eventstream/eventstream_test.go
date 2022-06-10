package eventstream_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/google/go-cmp/cmp"

	"github.com/tendermint/tendermint/internal/eventlog"
	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	rpccore "github.com/tendermint/tendermint/internal/rpc/core"
	"github.com/tendermint/tendermint/rpc/client/eventstream"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

func TestStream_filterOrder(t *testing.T) {
	defer leaktest.Check(t)

	s := newStreamTester(t, `tm.event = 'good'`, eventlog.LogSettings{
		WindowSize: 30 * time.Second,
	}, nil)

	// Verify that events are delivered in forward time order (i.e., that the
	// stream unpacks the pages correctly) and that events not matching the
	// query (here, type="bad") are skipped.
	//
	// The minimum batch size is 16 and half the events we publish match, so we
	// publish > 32 items (> 16 good) to ensure we exercise paging.
	etype := [2]string{"good", "bad"}
	var items []testItem
	for i := 0; i < 40; i++ {
		s.advance(100 * time.Millisecond)
		text := fmt.Sprintf("item%d", i)
		cur := s.publish(etype[i%2], text)

		// Even-numbered items match the target type.
		if i%2 == 0 {
			items = append(items, makeTestItem(cur, text))
		}
	}

	s.start()
	for _, itm := range items {
		s.mustItem(t, itm)
	}
	s.stopWait()
}

func TestStream_lostItem(t *testing.T) {
	defer leaktest.Check(t)

	s := newStreamTester(t, ``, eventlog.LogSettings{
		WindowSize: 30 * time.Second,
	}, nil)

	// Publish an item and let the client observe it.
	cur := s.publish("ok", "whatever")
	s.start()
	s.mustItem(t, makeTestItem(cur, "whatever"))
	s.stopWait()

	// Time passes, and cur expires out of the window.
	s.advance(50 * time.Second)
	next1 := s.publish("ok", "more stuff")
	s.advance(15 * time.Second)
	next2 := s.publish("ok", "still more stuff")

	// At this point, the oldest item in the log is newer than the point at
	// which we continued, we should get an error.
	s.start()
	var missed *eventstream.MissedItemsError
	if err := s.mustError(t); !errors.As(err, &missed) {
		t.Errorf("Wrong error: got %v, want %T", err, missed)
	} else {
		t.Logf("Correctly reported missed item: %v", missed)
	}

	// If we reset the stream and continue from head, we should catch up.
	s.stopWait()
	s.stream.Reset()
	s.start()

	s.mustItem(t, makeTestItem(next1, "more stuff"))
	s.mustItem(t, makeTestItem(next2, "still more stuff"))
	s.stopWait()
}

func TestMinPollTime(t *testing.T) {
	defer leaktest.Check(t)

	s := newStreamTester(t, ``, eventlog.LogSettings{
		WindowSize: 30 * time.Second,
	}, nil)

	s.publish("bad", "whatever")

	// Waiting for an item on a log with no matching events incurs a minimum
	// wait time and reports no events.
	ctx := context.Background()
	filter := &coretypes.EventFilter{Query: `tm.event = 'good'`}

	t.Run("NoneMatch", func(t *testing.T) {
		start := time.Now()

		// Request a very short delay, and affirm we got the server's minimum.
		rsp, err := s.env.Events(ctx, &coretypes.RequestEvents{
			Filter:   filter,
			MaxItems: 1,
			WaitTime: 10 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Events failed: %v", err)
		} else if elapsed := time.Since(start); elapsed < time.Second {
			t.Errorf("Events returned too quickly: got %v, wanted 1s", elapsed)
		} else if len(rsp.Items) != 0 {
			t.Errorf("Events returned %d items, expected none", len(rsp.Items))
		}
	})

	s.publish("good", "whatever")

	// Waiting for an available matching item incurs no delay.
	t.Run("SomeMatch", func(t *testing.T) {
		start := time.Now()

		// Request a long-ish delay and affirm we don't block for it.
		// Check for this by ensuring we return sooner than the minimum delay,
		// since we don't know the exact timing.
		rsp, err := s.env.Events(ctx, &coretypes.RequestEvents{
			Filter:   filter,
			MaxItems: 1,
			WaitTime: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("Events failed: %v", err)
		} else if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Errorf("Events returned too slowly: got %v, wanted immediate", elapsed)
		} else if len(rsp.Items) == 0 {
			t.Error("Events returned no items, wanted at least 1")
		}
	})
}

// testItem is a wrapper for comparing item results in a friendly output format
// for the cmp package.
type testItem struct {
	Cursor string
	Data   string

	// N.B. Fields exported to simplify use in cmp.
}

func makeTestItem(cur, data string) testItem {
	return testItem{
		Cursor: cur,
		Data:   fmt.Sprintf(`{"type":%q,"value":%q}`, types.EventDataString("").TypeTag(), data),
	}
}

// streamTester is a simulation harness for an eventstream.Stream. It simulates
// the production service by plumbing an event log into a stub RPC environment,
// into which the test can publish events and advance the perceived time to
// exercise various cases of the stream.
type streamTester struct {
	log    *eventlog.Log
	env    *rpccore.Environment
	clock  int64
	index  int64
	stream *eventstream.Stream
	errc   chan error
	recv   chan *coretypes.EventItem
	stop   func()
}

func newStreamTester(t *testing.T, query string, logOpts eventlog.LogSettings, streamOpts *eventstream.StreamOptions) *streamTester {
	t.Helper()
	s := new(streamTester)

	// Plumb a time source controlled by the tester into the event log.
	logOpts.Source = cursor.Source{
		TimeIndex: s.timeNow,
	}
	lg, err := eventlog.New(logOpts)
	if err != nil {
		t.Fatalf("Creating event log: %v", err)
	}
	s.log = lg
	s.env = &rpccore.Environment{EventLog: lg}
	s.stream = eventstream.New(s, query, streamOpts)
	return s
}

// start starts the stream receiver, which runs until it it terminated by
// calling stop.
func (s *streamTester) start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.errc = make(chan error, 1)
	s.recv = make(chan *coretypes.EventItem)
	s.stop = cancel
	go func() {
		defer close(s.errc)
		s.errc <- s.stream.Run(ctx, func(itm *coretypes.EventItem) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case s.recv <- itm:
				return nil
			}
		})
	}()
}

// publish adds a single event to the event log at the present moment.
func (s *streamTester) publish(etype, payload string) string {
	_ = s.log.Add(etype, types.EventDataString(payload))
	s.index++
	return fmt.Sprintf("%016x-%04x", s.clock, s.index)
}

// wait blocks until either an item is received or the runner stops.
func (s *streamTester) wait() (*coretypes.EventItem, error) {
	select {
	case itm := <-s.recv:
		return itm, nil
	case err := <-s.errc:
		return nil, err
	}
}

// mustItem waits for an item and fails if either an error occurs or the item
// does not match want.
func (s *streamTester) mustItem(t *testing.T, want testItem) {
	t.Helper()

	itm, err := s.wait()
	if err != nil {
		t.Fatalf("Receive: got error %v, want item %v", err, want)
	}
	got := testItem{Cursor: itm.Cursor, Data: string(itm.Data)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Item: (-want, +got)\n%s", diff)
	}
}

// mustError waits for an error and fails if an item is returned.
func (s *streamTester) mustError(t *testing.T) error {
	t.Helper()
	itm, err := s.wait()
	if err == nil {
		t.Fatalf("Receive: got item %v, want error", itm)
	}
	return err
}

// stopWait stops the runner and waits for it to terminate.
func (s *streamTester) stopWait() { s.stop(); s.wait() } //nolint:errcheck

// timeNow reports the current simulated time index.
func (s *streamTester) timeNow() int64 { return s.clock }

// advance moves the simulated time index.
func (s *streamTester) advance(d time.Duration) { s.clock += int64(d) }

// Events implements the eventstream.Client interface by delegating to a stub
// environment as if it were a local RPC client.  This works because the Events
// method only requires the event log, the other fields are unused.
func (s *streamTester) Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error) {
	return s.env.Events(ctx, req)
}
