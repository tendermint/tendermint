package p2p

import (
	"container/heap"
	"context"
	"sort"
	"time"

	"github.com/gogo/protobuf/proto"
)

type simpleQueue struct {
	input   chan Envelope
	output  chan Envelope
	closeFn func()
	closeCh <-chan struct{}

	maxSize int
	chDescs []*ChannelDescriptor
}

func newSimplePriorityQueue(ctx context.Context, size int, chDescs []*ChannelDescriptor) *simpleQueue {
	ctx, cancel := context.WithCancel(ctx)
	q := &simpleQueue{
		input:   make(chan Envelope, size*2),
		output:  make(chan Envelope, size/2),
		maxSize: size * size,
		closeCh: ctx.Done(),
		closeFn: cancel,
	}

	q.run(ctx)
	return q
}

func (q *simpleQueue) enqueue() chan<- Envelope { return q.input }
func (q *simpleQueue) dequeue() <-chan Envelope { return q.output }
func (q *simpleQueue) close()                   { q.closeFn() }
func (q *simpleQueue) closed() <-chan struct{}  { return q.closeCh }
func (q *simpleQueue) run(ctx context.Context) {
	defer q.closeFn()

	var chPriorities = make(map[ChannelID]uint, len(q.chDescs))
	for _, chDesc := range q.chDescs {
		chID := chDesc.ID
		chPriorities[chID] = uint(chDesc.Priority)
	}

	pq := make(priorityQueue, q.maxSize)
	heap.Init(&pq)
	ticker := time.NewTicker(10 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.closeCh:
			return
		case e := <-q.input:
			// enqueue the incoming Envelope
			heap.Push(&pq, pqEnvelope{
				envelope:  e,
				size:      uint(proto.Size(e.Message)),
				priority:  chPriorities[e.ChannelID],
				timestamp: time.Now().UTC(),
			})
			if len(pq) > q.maxSize {
				sort.Sort(pq)
				pq = pq[:q.maxSize]
			}
		case <-ticker.C:
			if len(pq) > q.maxSize {
				sort.Sort(pq)
				pq = pq[:q.maxSize]
			}
		case q.output <- heap.Pop(&pq).(pqEnvelope).envelope:
			// send to the output channel. popping
			// a potentially empty heap into an
			// empty channel might be suspect.
		}
	}
}
