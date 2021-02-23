package p2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
)

type drrFlow struct {
	mtx    tmsync.RWMutex
	size   uint
	buffer []Envelope

	capacity uint
	quantum  uint
	deficit  uint
}

// tryEnqueue attempts to enqueue an Envelope into the flow's buffer if there is
// sufficient capacity. If there is sufficient capacity, the flow's size is
// incremented by the Envelope's message size and the Envelope is added to the
// buffer. A boolean indicating if the Envelope was enqueued or not and the size
// of the flow is returned.
func (f *drrFlow) tryEnqueue(e Envelope) (bool, uint) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	msgSize := uint(proto.Size(e.Message))
	if f.size+msgSize > f.capacity {
		return false, f.size
	}

	f.buffer = append(f.buffer, e)
	f.size += msgSize

	return true, f.size
}

// resetDeficit sets the flow's deficit counter to zero.
func (f *drrFlow) resetDeficit() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	f.deficit = 0
}

// incrDeficit increases the current deficit by the flow's quantum.
func (f *drrFlow) incrDeficit() uint {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	f.deficit += f.quantum
	return f.deficit
}

// pop removes and returns the Envelope at the HoQ, head of the queue. It is
// assumed the flow's buffer is non-empty.
func (f *drrFlow) pop() Envelope {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	e := f.buffer[0]
	msgSize := uint(proto.Size(e.Message))

	f.deficit -= msgSize
	f.size -= msgSize
	f.buffer = f.buffer[1:]

	return e
}

// peek returns the Envelope at the HoQ, head of the queue. If the flow's buffer
// is empty, an empty Envelope is returned.
func (f *drrFlow) peek() Envelope {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

	if f.size == 0 || len(f.buffer) == 0 {
		return Envelope{}
	}

	return f.buffer[0]
}

func (f *drrFlow) getDeficit() uint {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

	return f.deficit
}

func (f *drrFlow) getCapacity() uint {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

	return f.capacity
}

func (f *drrFlow) getSize() uint {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

	return f.size
}

// assert the WDDR queue implements the queue interface at compile-time
var _ queue = (*wdrrQueue)(nil)

// wdrrQueue implements a Weighted Deficit Round Robin, WDRR, scheduling
// algorithm via the queue interface. A WDRR queue is created per peer, where
// the queue will have N number of flows. Each flow corresponds to a p2p Channel,
// so there are n input flows and a single output source, the peer's connection.
//
// Each flow has the following:
// - fixed capacity: The number of bytes it can hold. Note, partial messages are
//   not stored, so if a message exceeds the current size if the flow, even by a
//   single byte, it will be dropped.
// - size: The current number of bytes enqueued in the flow.
// - quantum: The number of bytes that is added to the deficit counter of the
//   flow in each round. The flow can send at most quantum bytes at a time. Each
//   flow has its own unique quantum, which gives the queue its weighted nature.
//   A higher quantum corresponds to a higher weight/priority.
// - deficit counter: The number of bytes that the flow is allowed to transmit
//   when it is its turn.
//
// See: https://en.wikipedia.org/wiki/Deficit_round_robin
type wdrrQueue struct {
	logger  log.Logger
	peerID  NodeID
	metrics *Metrics
	chIDs   []ChannelID
	flows   map[ChannelID]*drrFlow

	closer        *tmsync.Closer
	enqueueCh     chan Envelope
	enqueueCloser *tmsync.Closer
	dequeueCh     chan Envelope
	dequeueCloser *tmsync.Closer
}

func newWDRRQueue(logger log.Logger, pID NodeID, m *Metrics, chDescs []ChannelDescriptor) *wdrrQueue {
	// Copy the channel IDs from the channel descriptors and create a WDRR flow
	// per ChannelDescriptor.
	chIDs := make([]ChannelID, len(chDescs))
	flows := make(map[ChannelID]*drrFlow)
	for i, chDesc := range chDescs {
		chID := ChannelID(chDesc.ID)
		chIDs[i] = chID

		capacity := chDesc.MaxSendCapacityBytes
		if capacity == 0 {
			capacity = chDesc.MaxSendBytes
		}

		flows[chID] = &drrFlow{
			capacity: capacity,
			quantum:  chDesc.MaxSendBytes,
			buffer:   make([]Envelope, 0),
		}
	}

	return &wdrrQueue{
		logger:  logger.With("peer", pID),
		peerID:  pID,
		metrics: m,
		chIDs:   chIDs,
		flows:   flows,

		closer:        tmsync.NewCloser(),
		enqueueCh:     make(chan Envelope),
		enqueueCloser: tmsync.NewCloser(),
		dequeueCh:     make(chan Envelope),
		dequeueCloser: tmsync.NewCloser(),
	}
}

// enqueue returns an unbuffered write-only channel which a producer can send on.
func (q *wdrrQueue) enqueue() chan<- Envelope {
	return q.enqueueCh
}

// dequeue returns an unbuffered read-only channel which a consumer can read from.
func (q *wdrrQueue) dequeue() <-chan Envelope {
	return q.dequeueCh
}

func (q *wdrrQueue) closed() <-chan struct{} {
	return q.closer.Done()
}

// close closes the WDRR queue. After this call enqueue() will block, so the
// caller must select on closed() as well to avoid blocking forever. The
// enqueue() and dequeue() along with the internal channels will not be closed.
// Note, close() will block until all externally spawned goroutines have exited.
func (q *wdrrQueue) close() {
	q.closer.Close()
	<-q.enqueueCloser.Done()
	<-q.dequeueCloser.Done()
}

// start starts the WDRR queue process. This must be called before the queue can
// start to process and accept Envelopes.
func (q *wdrrQueue) start() {
	go q.enqueueLoop()
	go q.dequeueLoop()
}

// enqueueLoop performs a blocking process where a peer's flow, i.e. a p2p Channel,
// can send an Envelope via the channel returned by enqueue(). This process
// blocks and continuously reads off of the enqueue channel. For every Envelope
// read, the corresponding flow is found via the Envelope's ChannelID. If the
// flow has capacity to accept the Envelope, it is enqueued to the flow's buffer.
// Otherwise, the Envelope is dropped.
func (q *wdrrQueue) enqueueLoop() {
	defer q.enqueueCloser.Close()

	for {
		select {
		case e := <-q.enqueueCh:
			flow := q.flows[e.channelID]

			enqueued, size := flow.tryEnqueue(e)
			if !enqueued {
				msgSize := uint(proto.Size(e.Message))
				// TODO: Record gauge metric for dropped envelope.
				q.logger.Debug(
					"dropped envelope",
					"ch_id", e.channelID,
					"flow_capacity", flow.getCapacity(),
					"flow_size", size,
					"msg_size", msgSize,
				)
			}

		case <-q.closer.Done():
			return
		}
	}
}

// dequeueLoop performs the main WDRR scheduling algorithm. It starts a blocking
// process where if the queue is not closed, it will continuously iterate over
// each flow and perform the WDRR scheduling algorithm.
//
// For each flow, if it is non-empty, its deficit counter is incremented by its
// quantum value. Then, the value of the deficit counter is a maximal amount of
// bytes that can be sent at this turn. If the deficit counter is greater than
// the envelopes's message size at the head of the queue (HoQ), this envelope
// can be sent and the value of the counter is decremented by the message's size.
// Then, the size of the next envelopes's message is compared to the counter
// value, etc. Once the flow is empty or the value of the counter is insufficient,
// the scheduler will skip to the next flow. If the flow is empty, the value of
// the deficit counter is reset to 0.
//
// Note, there is a single unbuffered dequeue channel, i.e. a single source, so
// all sends will block until received (i.e. sent on the peer connection).
func (q *wdrrQueue) dequeueLoop() {
	defer q.dequeueCloser.Close()

	for {
		select {
		case <-q.closer.Done():
			return

		default:
			for _, chID := range q.chIDs {
				flow := q.flows[chID]
				if flow.getSize() > 0 {
					d := flow.incrDeficit()
					e := flow.peek()
					msgSize := uint(proto.Size(e.Message))

					// while the flow is non-empty and we have capacity to send
					for flow.getSize() > 0 && d >= msgSize {
						// Send and dequeue, where the send is to a single source and will
						// block until received. Note, pop() will decrement the deficit.
						q.dequeueCh <- e
						_ = flow.pop()

						if flow.getSize() > 0 {
							d = flow.getDeficit()
							e = flow.peek()
							msgSize = uint(proto.Size(e.Message))
						}
					}

					if flow.getSize() == 0 {
						flow.resetDeficit()
					}
				}
			}
		}
	}
}
