package p2p

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
)

const defaultCapacity uint = 1048576 // 1MB

// wrappedEnvelope wraps a p2p Envelope with its precomputed size.
type wrappedEnvelope struct {
	envelope Envelope
	size     uint
}

// assert the WDDR scheduler implements the queue interface at compile-time
var _ queue = (*wdrrScheduler)(nil)

// wdrrQueue implements a Weighted Deficit Round Robin (WDRR) scheduling
// algorithm via the queue interface. A WDRR queue is created per peer, where
// the queue will have N number of flows. Each flow corresponds to a p2p Channel,
// so there are n input flows and a single output source, the peer's connection.
//
// The WDRR scheduler contains a shared buffer with a fixed capacity.
//
// Each flow has the following:
// - quantum: The number of bytes that is added to the deficit counter of the
//   flow in each round. The flow can send at most quantum bytes at a time. Each
//   flow has its own unique quantum, which gives the queue its weighted nature.
//   A higher quantum corresponds to a higher weight/priority. The quantum is
//   computed as MaxSendBytes * Priority.
// - deficit counter: The number of bytes that the flow is allowed to transmit
//   when it is its turn.
//
// See: https://en.wikipedia.org/wiki/Deficit_round_robin
type wdrrScheduler struct {
	logger       log.Logger
	metrics      *Metrics
	chDescs      []ChannelDescriptor
	capacity     uint
	size         uint
	chPriorities map[ChannelID]uint
	buffer       map[ChannelID][]wrappedEnvelope
	quanta       map[ChannelID]uint
	deficits     map[ChannelID]uint

	closer *tmsync.Closer
	doneCh *tmsync.Closer

	enqueueCh chan Envelope
	dequeueCh chan Envelope
}

func newWDRRScheduler(
	logger log.Logger,
	m *Metrics,
	chDescs []ChannelDescriptor,
	enqueueBuf, dequeueBuf, capacity uint,
) *wdrrScheduler {

	// copy each ChannelDescriptor and sort them by channel priority
	chDescsCopy := make([]ChannelDescriptor, len(chDescs))
	copy(chDescsCopy, chDescs)
	sort.Slice(chDescsCopy, func(i, j int) bool { return chDescsCopy[i].Priority > chDescsCopy[j].Priority })

	var (
		buffer       = make(map[ChannelID][]wrappedEnvelope)
		chPriorities = make(map[ChannelID]uint)
		quanta       = make(map[ChannelID]uint)
		deficits     = make(map[ChannelID]uint)
	)

	for _, chDesc := range chDescsCopy {
		chID := ChannelID(chDesc.ID)
		chPriorities[chID] = uint(chDesc.Priority)
		buffer[chID] = make([]wrappedEnvelope, 0)
		quanta[chID] = chDesc.MaxSendBytes * uint(chDesc.Priority)
	}

	return &wdrrScheduler{
		logger:       logger.With("queue", "wdrr"),
		metrics:      m,
		capacity:     capacity,
		chPriorities: chPriorities,
		chDescs:      chDescsCopy,
		buffer:       buffer,
		quanta:       quanta,
		deficits:     deficits,
		closer:       tmsync.NewCloser(),
		doneCh:       tmsync.NewCloser(),
		enqueueCh:    make(chan Envelope, enqueueBuf),
		dequeueCh:    make(chan Envelope, dequeueBuf),
	}
}

// enqueue returns an unbuffered write-only channel which a producer can send on.
func (s *wdrrScheduler) enqueue() chan<- Envelope {
	return s.enqueueCh
}

// dequeue returns an unbuffered read-only channel which a consumer can read from.
func (s *wdrrScheduler) dequeue() <-chan Envelope {
	return s.dequeueCh
}

func (s *wdrrScheduler) closed() <-chan struct{} {
	return s.closer.Done()
}

// close closes the WDRR queue. After this call enqueue() will block, so the
// caller must select on closed() as well to avoid blocking forever. The
// enqueue() and dequeue() along with the internal channels will NOT be closed.
// Note, close() will block until all externally spawned goroutines have exited.
func (s *wdrrScheduler) close() {
	s.closer.Close()
	<-s.doneCh.Done()
}

// start starts the WDRR queue process in a blocking goroutine. This must be
// called before the queue can start to process and accept Envelopes.
func (s *wdrrScheduler) start() {
	go s.process()
}

// process starts a blocking WDRR scheduler process, where we continuously
// evaluate if we need to attempt to enqueue an Envelope or schedule Envelopes
// to be dequeued and subsequently read and sent on the source connection.
// Internally, each p2p Channel maps to a flow, where each flow has a deficit
// and a quantum.
//
// For each Envelope requested to be enqueued, we evaluate if there is sufficient
// capacity in the shared buffer to add the Envelope. If so, it is added.
// Otherwise, we evaluate all flows of lower priority where we attempt find an
// existing Envelope in the shared buffer of sufficient size that can be dropped
// in place of the incoming Envelope. If there is no such Envelope that can be
// dropped, then the incoming Envelope is dropped.
//
// When there is nothing to be enqueued, we perform the WDRR algorithm and
// determine which Envelopes can be dequeued. For each Envelope that can be
// dequeued, it is sent on the dequeueCh. Specifically, for each flow, if it is
// non-empty, its deficit counter is incremented by its quantum value. Then, the
// value of the deficit counter is a maximal amount of bytes that can be sent at
// this round. If the deficit counter is greater than the Envelopes's message
// size at the head of the queue (HoQ), this envelope can be sent and the value
// of the counter is decremented by the message's size. Then, the size of the
// next Envelopes's message is compared to the counter value, etc. Once the flow
// is empty or the value of the counter is insufficient, the scheduler will skip
// to the next flow. If the flow is empty, the value of the deficit counter is
// reset to 0.
//
// XXX/TODO: Evaluate the single goroutine scheduler mechanism. In other words,
// evaluate the effectiveness and performance of having a single goroutine
// perform handling both enqueueing and dequeueing logic. Specifically, there
// is potentially contention between reading off of enqueueCh and trying to
// enqueue while also attempting to perform the WDRR algorithm and find the next
// set of Envelope(s) to send on the dequeueCh. Alternatively, we could consider
// separate scheduling goroutines, but then that requires the use of mutexes and
// possibly a degrading performance.
func (s *wdrrScheduler) process() {
	defer s.doneCh.Close()

	for {
		select {
		case <-s.closer.Done():
			return

		case e := <-s.enqueueCh:
			// attempt to enqueue the incoming Envelope
			chIDStr := strconv.Itoa(int(e.channelID))
			wEnv := wrappedEnvelope{envelope: e, size: uint(proto.Size(e.Message))}
			msgSize := wEnv.size

			s.metrics.PeerPendingSendBytes.With("peer_id", string(e.To)).Add(float64(msgSize))

			// If we're at capacity, we need to either drop the incoming Envelope or
			// an Envelope from a lower priority flow. Otherwise, we add the (wrapped)
			// envelope to the flow's queue.
			if s.size+wEnv.size > s.capacity {
				chPriority := s.chPriorities[e.channelID]

				var (
					canDrop  bool
					dropIdx  int
					dropChID ChannelID
				)

				// Evaluate all lower priority flows and determine if there exists an
				// Envelope that is of equal or greater size that we can drop in favor
				// of the incoming Envelope.
				for i := len(s.chDescs) - 1; i >= 0 && uint(s.chDescs[i].Priority) < chPriority && !canDrop; i-- {
					currChID := ChannelID(s.chDescs[i].ID)
					flow := s.buffer[currChID]

					for j := 0; j < len(flow) && !canDrop; j++ {
						if flow[j].size >= wEnv.size {
							canDrop = true
							dropIdx = j
							dropChID = currChID
							break
						}
					}
				}

				// If we can drop an existing Envelope, drop it and enqueue the incoming
				// Envelope.
				if canDrop {
					chIDStr = strconv.Itoa(int(dropChID))
					chPriority = s.chPriorities[dropChID]
					msgSize = s.buffer[dropChID][dropIdx].size

					// Drop Envelope for the lower priority flow and update the queue's
					// buffer size
					s.size -= msgSize
					s.buffer[dropChID] = append(s.buffer[dropChID][:dropIdx], s.buffer[dropChID][dropIdx+1:]...)

					// add the incoming Envelope and update queue's buffer size
					s.size += wEnv.size
					s.buffer[e.channelID] = append(s.buffer[e.channelID], wEnv)
					s.metrics.PeerQueueMsgSize.With("ch_id", chIDStr).Set(float64(wEnv.size))
				}

				// We either dropped the incoming Enevelope or one from an existing
				// lower priority flow.
				s.metrics.PeerQueueDroppedMsgs.With("ch_id", chIDStr).Add(1)
				s.logger.Debug(
					"dropped envelope",
					"ch_id", chIDStr,
					"priority", chPriority,
					"capacity", s.capacity,
					"msg_size", msgSize,
				)
			} else {
				// we have sufficient capacity to enqueue the incoming Envelope
				s.metrics.PeerQueueMsgSize.With("ch_id", chIDStr).Set(float64(wEnv.size))
				s.buffer[e.channelID] = append(s.buffer[e.channelID], wEnv)
				s.size += wEnv.size
			}

		default:
			// perform the WDRR algorithm
			for _, chDesc := range s.chDescs {
				chID := ChannelID(chDesc.ID)

				// only consider non-empty flows
				if len(s.buffer[chID]) > 0 {
					// bump flow's quantum
					s.deficits[chID] += s.quanta[chID]

					// grab the flow's current deficit counter and HoQ (wrapped) Envelope
					d := s.deficits[chID]
					we := s.buffer[chID][0]

					// While the flow is non-empty and we can send the current Envelope
					// on the dequeueCh:
					//
					// 1. send the Envelope
					// 2. update the scheduler's shared buffer's size
					// 3. update the flow's deficit
					// 4. remove from the flow's queue
					// 5. grab the next HoQ Envelope and flow's deficit
					for len(s.buffer[chID]) > 0 && d >= we.size {
						s.metrics.PeerSendBytesTotal.With(
							"chID", fmt.Sprint(chID),
							"peer_id", string(we.envelope.To)).Add(float64(we.size))
						s.dequeueCh <- we.envelope
						s.size -= we.size
						s.deficits[chID] -= we.size
						s.buffer[chID] = s.buffer[chID][1:]

						if len(s.buffer[chID]) > 0 {
							d = s.deficits[chID]
							we = s.buffer[chID][0]
						}
					}
				}

				// reset the flow's deficit to zero if it is empty
				if len(s.buffer[chID]) == 0 {
					s.deficits[chID] = 0
				}
			}
		}
	}
}
