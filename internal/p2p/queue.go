package p2p

import "context"

// default capacity for the size of a queue
const defaultCapacity uint = 16e6 // ~16MB

// queue does QoS scheduling for Envelopes, enqueueing and dequeueing according
// to some policy. Queues are used at contention points, i.e.:
//
// - Receiving inbound messages to a single channel from all peers.
// - Sending outbound messages to a single peer from all channels.
type queue interface {
	// enqueue returns a channel for submitting envelopes.
	enqueue() chan<- Envelope

	// dequeue returns a channel ordered according to some queueing policy.
	dequeue() <-chan Envelope

	close()
	closed() <-chan struct{}
}

// fifoQueue is a simple unbuffered lossless queue that passes messages through
// in the order they were received, and blocks until message is received.
type fifoQueue struct {
	queueCh chan Envelope
	signal  chan struct{}
}

func newFIFOQueue(_ context.Context, size int) queue {
	return &fifoQueue{
		queueCh: make(chan Envelope, size),
		signal:  make(chan struct{}),
	}
}

func (q *fifoQueue) enqueue() chan<- Envelope {
	return q.queueCh
}

func (q *fifoQueue) dequeue() <-chan Envelope {
	return q.queueCh
}

func (q *fifoQueue) close() {
	select {
	case <-q.signal:
		return
	default:
		close(q.signal)
	}

}
func (q *fifoQueue) closed() <-chan struct{} { return q.signal }
