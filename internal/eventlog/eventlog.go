// Package eventlog defines a reverse time-ordered log of events over a sliding
// window of time before the most recent item in the log.
//
// New items are added to the head of the log (the newest end), and items that
// fall outside the designated window are pruned from its tail (the oldest).
// Items within the log are indexed by lexicographically-ordered cursors.
package eventlog

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	"github.com/tendermint/tendermint/types"
)

// A Log is a reverse time-ordered log of events in a sliding window of time
// before the newest item. Use Add to add new items to the front (head) of the
// log, and Scan or WaitScan to traverse the current contents of the log.
//
// After construction, a *Log is safe for concurrent access by one writer and
// any number of readers.
type Log struct {
	// These values do not change after construction.
	windowSize time.Duration
	maxItems   int
	metrics    *Metrics

	// Protects access to the fields below.  Lock to modify the values of these
	// fields, or to read or snapshot the values.
	mu sync.Mutex

	numItems     int           // total number of items in the log
	oldestCursor cursor.Cursor // cursor of the oldest item
	head         *logEntry     // pointer to the newest item
	ready        chan struct{} // closed when head changes
	source       cursor.Source // generator of cursors
}

// New constructs a new empty log with the given settings.
func New(opts LogSettings) (*Log, error) {
	if opts.WindowSize <= 0 {
		return nil, errors.New("window size must be positive")
	}
	lg := &Log{
		windowSize: opts.WindowSize,
		maxItems:   opts.MaxItems,
		metrics:    NopMetrics(),
		ready:      make(chan struct{}),
		source:     opts.Source,
	}
	if opts.Metrics != nil {
		lg.metrics = opts.Metrics
	}
	return lg, nil
}

// Add adds a new item to the front of the log. If necessary, the log is pruned
// to fit its constraints on size and age. Add blocks until both steps are done.
//
// Any error reported by Add arises from pruning; the new item was added to the
// log regardless whether an error occurs.
func (lg *Log) Add(etype string, data types.EventData) error {
	lg.mu.Lock()
	head := &logEntry{
		item: newItem(lg.source.Cursor(), etype, data),
		next: lg.head,
	}
	lg.numItems++
	lg.updateHead(head)
	size := lg.numItems
	age := head.item.Cursor.Diff(lg.oldestCursor)

	// If the log requires pruning, do the pruning step outside the lock.  This
	// permits readers to continue to make progress while we're working.
	lg.mu.Unlock()
	return lg.checkPrune(head, size, age)
}

// Scan scans the current contents of the log, calling f with each item until
// all items are visited or f reports an error. If f returns ErrStopScan, Scan
// returns nil, otherwise it returns the error reported by f.
//
// The Info value returned is valid even if Scan reports an error.
func (lg *Log) Scan(f func(*Item) error) (Info, error) {
	return lg.scanState(lg.state(), f)
}

// WaitScan blocks until the cursor of the frontmost log item is different from
// c, then executes a Scan on the contents of the log. If ctx ends before the
// head is updated, WaitScan returns an error without calling f.
//
// The Info value returned is valid even if WaitScan reports an error.
func (lg *Log) WaitScan(ctx context.Context, c cursor.Cursor, f func(*Item) error) (Info, error) {
	st := lg.state()
	for st.head == nil || st.head.item.Cursor == c {
		var err error
		st, err = lg.waitStateChange(ctx)
		if err != nil {
			return st.info(), err
		}
	}
	return lg.scanState(st, f)
}

// Info returns the current state of the log.
func (lg *Log) Info() Info { return lg.state().info() }

// ErrStopScan is returned by a Scan callback to signal that scanning should be
// terminated without error.
var ErrStopScan = errors.New("stop scanning")

// ErrLogPruned is returned by Add to signal that at least some events within
// the time window were discarded by pruning in excess of the size limit.
// This error may be wrapped, use errors.Is to test for it.
var ErrLogPruned = errors.New("log pruned")

// LogSettings configure the construction of an event log.
type LogSettings struct {
	// The size of the time window measured in time before the newest item.
	// This value must be positive.
	WindowSize time.Duration

	// The maximum number of items that will be retained in memory within the
	// designated time window. A value ≤ 0 imposes no limit, otherwise items in
	// excess of this number will be dropped from the log.
	MaxItems int

	// The cursor source to use for log entries. If not set, use wallclock time.
	Source cursor.Source

	// If non-nil, exported metrics to update. If nil, metrics are discarded.
	Metrics *Metrics
}

// Info records the current state of the log at the time of a scan operation.
type Info struct {
	Oldest cursor.Cursor // the cursor of the oldest item in the log
	Newest cursor.Cursor // the cursor of the newest item in the log
	Size   int           // the number of items in the log
}

// logState is a snapshot of the state of the log.
type logState struct {
	oldest cursor.Cursor
	newest cursor.Cursor
	size   int
	head   *logEntry
}

func (st logState) info() Info {
	return Info{Oldest: st.oldest, Newest: st.newest, Size: st.size}
}

// state returns a snapshot of the current log contents. The caller may freely
// traverse the internal structure of the list without locking, provided it
// does not modify either the entries or their items.
func (lg *Log) state() logState {
	lg.mu.Lock()
	defer lg.mu.Unlock()
	if lg.head == nil {
		return logState{} // empty
	}
	return logState{
		oldest: lg.oldestCursor,
		newest: lg.head.item.Cursor,
		size:   lg.numItems,
		head:   lg.head,
	}
}

// waitStateChange blocks until either ctx ends or the head of the log is
// modified, then returns the state of the log.  An error is reported only if
// ctx terminates before head changes.
func (lg *Log) waitStateChange(ctx context.Context) (logState, error) {
	lg.mu.Lock()
	ch := lg.ready // capture
	lg.mu.Unlock()
	select {
	case <-ctx.Done():
		return lg.state(), ctx.Err()
	case <-ch:
		return lg.state(), nil
	}
}

// scanState scans the contents of the log at st.  See the Scan method for a
// description of the callback semantics.
func (lg *Log) scanState(st logState, f func(*Item) error) (Info, error) {
	info := Info{Oldest: st.oldest, Newest: st.newest, Size: st.size}
	for cur := st.head; cur != nil; cur = cur.next {
		if err := f(cur.item); err != nil {
			if errors.Is(err, ErrStopScan) {
				return info, nil
			}
			return info, err
		}
	}
	return info, nil
}

// updateHead replaces the current head with newHead, signals any waiters, and
// resets the wait signal. The caller must hold log.mu exclusively.
func (lg *Log) updateHead(newHead *logEntry) {
	lg.head = newHead
	close(lg.ready) // signal
	lg.ready = make(chan struct{})
}

// A logEntry is the backbone of the event log queue. Entries are not mutated
// after construction, so it is safe to read item and next without locking.
type logEntry struct {
	item *Item
	next *logEntry
}
