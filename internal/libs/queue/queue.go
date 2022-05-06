// Package queue implements a dynamic FIFO queue with a fixed upper bound
// and a flexible quota mechanism to handle bursty load.
package queue

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrQueueFull is returned by the Add method of a queue when the queue has
	// reached its hard capacity limit.
	ErrQueueFull = errors.New("queue is full")

	// ErrNoCredit is returned by the Add method of a queue when the queue has
	// exceeded its soft quota and there is insufficient burst credit.
	ErrNoCredit = errors.New("insufficient burst credit")

	// ErrQueueClosed is returned by the Add method of a closed queue, and by
	// the Wait method of a closed empty queue.
	ErrQueueClosed = errors.New("queue is closed")

	// Sentinel errors reported by the New constructor.
	errHardLimit   = errors.New("hard limit must be > 0 and ≥ soft quota")
	errBurstCredit = errors.New("burst credit must be non-negative")
)

// A Queue is a limited-capacity FIFO queue of arbitrary data items.
//
// A queue has a soft quota and a hard limit on the number of items that may be
// contained in the queue. Adding items in excess of the hard limit will fail
// unconditionally.
//
// For items in excess of the soft quota, a credit system applies: Each queue
// maintains a burst credit score. Adding an item in excess of the soft quota
// costs 1 unit of burst credit. If there is not enough burst credit, the add
// will fail.
//
// The initial burst credit is assigned when the queue is constructed. Removing
// items from the queue adds additional credit if the resulting queue length is
// less than the current soft quota. Burst credit is capped by the hard limit.
//
// A Queue is safe for concurrent use by multiple goroutines.
type Queue struct {
	mu sync.Mutex // protects the fields below

	softQuota int     // adjusted dynamically (see Add, Remove)
	hardLimit int     // fixed for the lifespan of the queue
	queueLen  int     // number of entries in the queue list
	credit    float64 // current burst credit

	closed bool
	nempty *sync.Cond
	back   *entry
	front  *entry

	// The queue is singly-linked. Front points to the sentinel and back points
	// to the newest entry. The oldest entry is front.link if it exists.
}

// New constructs a new empty queue with the specified options.  It reports an
// error if any of the option values are invalid.
func New(opts Options) (*Queue, error) {
	if opts.HardLimit <= 0 || opts.HardLimit < opts.SoftQuota {
		return nil, errHardLimit
	}
	if opts.BurstCredit < 0 {
		return nil, errBurstCredit
	}
	if opts.SoftQuota <= 0 {
		opts.SoftQuota = opts.HardLimit
	}
	if opts.BurstCredit == 0 {
		opts.BurstCredit = float64(opts.SoftQuota)
	}
	sentinel := new(entry)
	q := &Queue{
		softQuota: opts.SoftQuota,
		hardLimit: opts.HardLimit,
		credit:    opts.BurstCredit,
		back:      sentinel,
		front:     sentinel,
	}
	q.nempty = sync.NewCond(&q.mu)
	return q, nil
}

// Add adds item to the back of the queue. It reports an error and does not
// enqueue the item if the queue is full or closed, or if it exceeds its soft
// quota and there is not enough burst credit.
func (q *Queue) Add(item interface{}) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	if q.queueLen >= q.softQuota {
		if q.queueLen == q.hardLimit {
			return ErrQueueFull
		} else if q.credit < 1 {
			return ErrNoCredit
		}

		// Successfully exceeding the soft quota deducts burst credit and raises
		// the soft quota. This has the effect of reducing the credit cap and the
		// amount of credit given for removing items to better approximate the
		// rate at which the consumer is servicing the queue.
		q.credit--
		q.softQuota = q.queueLen + 1
	}
	e := &entry{item: item}
	q.back.link = e
	q.back = e
	q.queueLen++
	if q.queueLen == 1 { // was empty
		q.nempty.Signal()
	}
	return nil
}

// Remove removes and returns the frontmost (oldest) item in the queue and
// reports whether an item was available.  If the queue is empty, Remove
// returns nil, false.
func (q *Queue) Remove() (interface{}, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.queueLen == 0 {
		return nil, false
	}
	return q.popFront(), true
}

// Wait blocks until q is non-empty or closed, and then returns the frontmost
// (oldest) item from the queue. If ctx ends before an item is available, Wait
// returns a nil value and a context error. If the queue is closed while it is
// still empty, Wait returns nil, ErrQueueClosed.
func (q *Queue) Wait(ctx context.Context) (interface{}, error) {
	// If the context terminates, wake the waiter.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); q.nempty.Broadcast() }()

	q.mu.Lock()
	defer q.mu.Unlock()

	for q.queueLen == 0 {
		if q.closed {
			return nil, ErrQueueClosed
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			q.nempty.Wait()
		}
	}
	return q.popFront(), nil
}

// Close closes the queue. After closing, any further Add calls will report an
// error, but items that were added to the queue prior to closing will still be
// available for Remove and Wait. Wait will report an error without blocking if
// it is called on a closed, empty queue.
func (q *Queue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.nempty.Broadcast()
	return nil
}

// popFront removes the frontmost item of q and returns its value after
// updating quota and credit settings.
//
// Preconditions: The caller holds q.mu and q is not empty.
func (q *Queue) popFront() interface{} {
	e := q.front.link
	q.front.link = e.link
	if e == q.back {
		q.back = q.front
	}
	q.queueLen--

	if q.queueLen < q.softQuota {
		// Successfully removing items from the queue below half the soft quota
		// lowers the soft quota. This has the effect of increasing the credit cap
		// and the amount of credit given for removing items to better approximate
		// the rate at which the consumer is servicing the queue.
		if q.softQuota > 1 && q.queueLen < q.softQuota/2 {
			q.softQuota--
		}

		// Give credit for being below the soft quota. Note we do this after
		// adjusting the quota so the credit reflects the item we just removed.
		q.credit += float64(q.softQuota-q.queueLen) / float64(q.softQuota)
		if cap := float64(q.hardLimit - q.softQuota); q.credit > cap {
			q.credit = cap
		}
	}

	return e.item
}

// Options are the initial settings for a Queue.
type Options struct {
	// The maximum number of items the queue will ever be permitted to hold.
	// This value must be positive, and greater than or equal to SoftQuota. The
	// hard limit is fixed and does not change as the queue is used.
	//
	// The hard limit should be chosen to exceed the largest burst size expected
	// under normal operating conditions.
	HardLimit int

	// The initial expected maximum number of items the queue should contain on
	// an average workload. If this value is zero, it is initialized to the hard
	// limit. The soft quota is adjusted from the initial value dynamically as
	// the queue is used.
	SoftQuota int

	// The initial burst credit score.  This value must be greater than or equal
	// to zero. If it is zero, the soft quota is used.
	BurstCredit float64
}

type entry struct {
	item interface{}
	link *entry
}
