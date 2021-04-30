package statesync

import (
	"sort"
	"sync"
	"time"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type lightBlockResponse struct {
	block *types.LightBlock
	peer  p2p.NodeID
}

// a block queue is used for asynchronously fetching and verifying light blocks
type blockQueue struct {
	mtx sync.Mutex

	// cursors to keep track of which heights need to be fetched and verified
	fetchHeight  int64
	verifyHeight int64

	// termination conditions
	stopHeight int64
	stopTime   time.Time

	// track failed heights (ordered from height to lowest) so we know what
	// blocks to try fetch again
	failed  []int64
	last    *lightBlockResponse
	pending map[int64]lightBlockResponse
	waiters map[int64]chan lightBlockResponse
}

func newBlockQueue(
	startHeight, stopHeight int64,
	stopTime time.Time,
) *blockQueue {
	return &blockQueue{
		stopHeight:   stopHeight,
		stopTime:     stopTime,
		fetchHeight:  startHeight,
		verifyHeight: startHeight,
	}
}

// Add adds a block to the queue to be verified and stored
func (q *blockQueue) Add(l lightBlockResponse) {
	if ch, ok := q.waiters[l.block.Height]; ok {
		ch <- l
		close(ch)
	} else {
		q.pending[l.block.Height] = l
	}
	q.last = &l

}

// NextHeight returns the next height that needs to be retrieved
func (q *blockQueue) NextHeight() int64 {
	height := q.fetchHeight
	// if a previous process failed then we pick up this one
	if len(q.failed) > 0 {
		height = q.failed[0]
		if len(q.failed) > 1 {
			q.failed = q.failed[1:]
		} else {
			q.failed = []int64{}
		}
	} else {
		// else we decrement the fetch height (move down to the next height)
		q.fetchHeight--
	}
	return height
}

// Finished returns true when the block queue has has all light blocks retrieved,
// verified and stored. There is no more work left to be done
func (q *blockQueue) Finished() bool {
	// if we haven't retrieved all the blocks yet then we can't be finished
	if !q.Complete() {
		return false
	}

	// if we don't have any more blocks waiting to be verified and stored then
	// we are done
	return len(q.pending) == 0
}

// Complete returns true when all the light blocks have been fetched and no more
// are needed.
// TODO: We need to keep an eye out for the rare condition that none of the
// peers have the blocks at the height we need. Thus we should have some form of
// global timeout or max retries
func (q *blockQueue) Complete() bool {
	// if there are some requests that failed, they need to be retried
	if len(q.failed) > 0 {
		return false
	}

	if q.last == nil {
		return false
	}

	// the queue is considered complete when the last block received has a
	// height that is less than or equal to the stop height and the last block
	// time is before the stop time. We now have all the data to prove any
	// inbound evidence
	return q.last.block.Height <= q.stopHeight && q.last.block.Time.Before(q.stopTime)
}

// VerifyNext pulls the next block off the pending queue and adds it to a
// channel if it's already there or creates a waiter to add it to the
// channel once it comes in. This is assumed to
// be a single thread as light blocks need to be sequentially verified.
func (q *blockQueue) VerifyNext() <-chan lightBlockResponse {
	ch := make(chan lightBlockResponse, 1)
	// check if we already have the light block waiting
	if lb, ok := q.pending[q.verifyHeight]; ok {
		delete(q.pending, q.verifyHeight)
		ch <- lb
		close(ch)
	} else {
		q.waiters[q.verifyHeight] = ch
	}

	// decrement the verify height (i.e. slide down the cursor)
	q.verifyHeight--

	return ch
}

// Retry is called when a dispatcher failed to fetch a light block or the
// fetched light block failed verification. It signals to the queue to add the
// height back to the request queue
func (q *blockQueue) Retry(height int64) {
	q.failed = insertSorted(q.failed, height)

	// in the case that a verification failed we need to revert back to the
	// previous height so that we can fetch and verify it again. This is needed
	// to ensure serialization
	if q.verifyHeight < height {
		q.verifyHeight = height
	}
}

// insertAt inserts v into s at index i and returns the new slice.
func insertAt(list []int64, i int, v int64) []int64 {
	if i == len(list) {
		// Insert at end is the easy case.
		return append(list, v)
	}

	// Make space for the new element
	list = append(list[:i+1], list[i:]...)

	// Insert the new element.
	list[i] = v

	// Return the updated slice.
	return list
}

func insertSorted(list []int64, v int64) []int64 {
	i := sort.Search(len(list), func(i int) bool { return list[i] >= v })
	return insertAt(list, i, v)
}
