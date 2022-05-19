package eventlog

import (
	"time"
)

// checkPrune checks whether the log has exceeded its boundaries of size or
// age, and if so prunes the log and updates the head.
func (lg *Log) checkPrune(head *logEntry, size int, age time.Duration) error {
	// To avoid potentially re-pruning for every event, don't trigger an age
	// prune until we're at least this far beyond the designated size.
	const windowSlop = 30 * time.Second

	if age < (lg.windowSize+windowSlop) && (lg.maxItems <= 0 || size <= lg.maxItems) {
		lg.metrics.numItems.Set(float64(lg.numItems))
		return nil // no pruning is needed
	}

	var newState logState
	var err error

	switch {
	case lg.maxItems > 0 && size > lg.maxItems:
		// We exceeded the size cap. In this case, age does not matter: count off
		// the newest items and drop the unconsumed tail. Note that we prune by a
		// fraction rather than an absolute amount so that we only have to prune
		// for size occasionally.

		// TODO(creachadair): We may want to spill dropped events to secondary
		// storage rather than dropping them. The size cap is meant as a safety
		// valve against unexpected extremes, but if a network has "expected"
		// spikes that nevertheless exceed any safe buffer size (e.g., Osmosis
		// epochs), we may want to have a fallback so that we don't lose events
		// that would otherwise fall within the window.
		newSize := 3 * size / 4
		newState, err = lg.pruneSize(head, newSize)

	default:
		// We did not exceed the size cap, but some items are too old.
		newState = lg.pruneAge(head)
	}

	// Note that when we update the head after pruning, we do not need to signal
	// any waiters; pruning never adds new material to the log so anyone waiting
	// should continue doing so until a subsequent Add occurs.
	lg.mu.Lock()
	defer lg.mu.Unlock()
	lg.numItems = newState.size
	lg.metrics.numItems.Set(float64(newState.size))
	lg.oldestCursor = newState.oldest
	lg.head = newState.head
	return err
}

// pruneSize returns a new log state by pruning head to newSize.
// Precondition: newSize ≤ len(head).
func (lg *Log) pruneSize(head *logEntry, newSize int) (logState, error) {
	// Special case for size 0 to simplify the logic below.
	if newSize == 0 {
		return logState{}, ErrLogPruned // drop everything
	}

	// Initialize: New head has the same item as the old head.
	first := &logEntry{item: head.item} // new head
	last := first                       // new tail (last copied cons)

	cur := head.next
	for i := 1; i < newSize; i++ {
		cp := &logEntry{item: cur.item}
		last.next = cp
		last = cp

		cur = cur.next
	}
	var err error
	if head.item.Cursor.Diff(last.item.Cursor) <= lg.windowSize {
		err = ErrLogPruned
	}

	return logState{
		oldest: last.item.Cursor,
		newest: first.item.Cursor,
		size:   newSize,
		head:   first,
	}, err
}

// pruneAge returns a new log state by pruning items older than the window
// prior to the head element.
func (lg *Log) pruneAge(head *logEntry) logState {
	first := &logEntry{item: head.item}
	last := first

	size := 1
	for cur := head.next; cur != nil; cur = cur.next {
		diff := head.item.Cursor.Diff(cur.item.Cursor)
		if diff > lg.windowSize {
			break // all remaining items are older than the window
		}
		cp := &logEntry{item: cur.item}
		last.next = cp
		last = cp
		size++
	}
	return logState{
		oldest: last.item.Cursor,
		newest: first.item.Cursor,
		size:   size,
		head:   first,
	}
}
