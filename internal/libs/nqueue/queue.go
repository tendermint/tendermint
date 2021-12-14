// Package nqueue implements a dynamic FIFO queue
package nqueue

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrQueueClosed is returned by the Add method of a closed queue, and by
	// the Wait method of a closed empty queue.
	ErrQueueClosed = errors.New("queue is closed")
)

// A Queue is a FIFO queue of arbitrary data items.
// A Queue is safe for concurrent use by multiple goroutines.
type Queue struct {
	mu sync.Mutex // protects the fields below

	queueLen int // number of entries in the queue list

	closed bool
	nempty *sync.Cond
	back   *entry
	front  *entry

	// The queue is singly-linked. Front points to the sentinel and back points
	// to the newest entry. The oldest entry is front.link if it exists.
}

// New constructs a new empty queue.
func New() *Queue {
	sentinel := new(entry)
	q := &Queue{
		back:  sentinel,
		front: sentinel,
	}
	q.nempty = sync.NewCond(&q.mu)
	return q
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
	return e.item
}

type entry struct {
	item interface{}
	link *entry
}
