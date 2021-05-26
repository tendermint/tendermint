package statesync

import (
	"fmt"
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
	terminal   *types.LightBlock

	// track failed heights (ordered from highest to lowest) so we know what
	// blocks to try fetch again
	failed     []int64
	retries    int
	maxRetries int

	// store inbound blocks and serve them to a verifying thread via a channel
	pending  map[int64]lightBlockResponse
	verifyCh chan lightBlockResponse

	// waiters are workers on idle until a height is required
	waiters []chan int64

	// this channel is closed once the verification process is complete
	doneCh chan struct{}
}

func newBlockQueue(
	startHeight, stopHeight int64,
	stopTime time.Time,
	maxRetries int,
) *blockQueue {
	return &blockQueue{
		stopHeight:   stopHeight,
		stopTime:     stopTime,
		fetchHeight:  startHeight,
		verifyHeight: startHeight,
		pending:      make(map[int64]lightBlockResponse),
		failed:       make([]int64, 0),
		retries:      0,
		maxRetries:   maxRetries,
		waiters:      make([]chan int64, 0),
		doneCh:       make(chan struct{}),
	}
}

// Add adds a block to the queue to be verified and stored
// CONTRACT: light blocks should have passed basic validation
func (q *blockQueue) add(l lightBlockResponse) {
	q.mtx.Lock()
	defer q.mtx.Unlock()

	// return early if the process has already finished
	select {
	case <-q.doneCh:
		return
	default:
	}

	// sometimes more blocks are fetched then what is necessary. If we already
	// have what we need then ignore this
	if q.terminal != nil && l.block.Height < q.terminal.Height {
		return
	}

	// if the block that was returned is at the verify height then the verifier
	// is already waiting for this block so we send it directly to them
	if l.block.Height == q.verifyHeight && q.verifyCh != nil {
		q.verifyCh <- l
		close(q.verifyCh)
		q.verifyCh = nil
	} else {
		// else we add it in the pending bucket
		q.pending[l.block.Height] = l
	}

	// Lastly, if the incoming block is past the stop time and stop height then
	// we mark it as the terminal block
	if l.block.Height <= q.stopHeight && l.block.Time.Before(q.stopTime) {
		q.terminal = l.block
	}
}

// NextHeight returns the next height that needs to be retrieved.
// We assume that for every height allocated that the peer will eventually add
// the block or signal that it needs to be retried
func (q *blockQueue) nextHeight() <-chan int64 {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	ch := make(chan int64, 1)
	// if a previous process failed then we pick up this one
	if len(q.failed) > 0 {
		failedHeight := q._nextFailed()
		ch <- failedHeight
		close(ch)
		return ch
	}

	if q.terminal == nil {
		// return and decrement the fetch height
		ch <- q.fetchHeight
		q.fetchHeight--
		close(ch)
		return ch
	}

	// at this point there is no height that we know we need so we create a
	// waiter to hold out for either an outgoing request to fail or a block to
	// fail verification
	q.waiters = append(q.waiters, ch)
	return ch
}

// Finished returns true when the block queue has has all light blocks retrieved,
// verified and stored. There is no more work left to be done
func (q *blockQueue) done() <-chan struct{} {
	return q.doneCh
}

// VerifyNext pulls the next block off the pending queue and adds it to a
// channel if it's already there or creates a waiter to add it to the
// channel once it comes in. NOTE: This is assumed to
// be a single thread as light blocks need to be sequentially verified.
func (q *blockQueue) verifyNext() <-chan lightBlockResponse {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	ch := make(chan lightBlockResponse, 1)

	select {
	case <-q.doneCh:
		return ch
	default:
	}

	if lb, ok := q.pending[q.verifyHeight]; ok {
		ch <- lb
		close(ch)
		delete(q.pending, q.verifyHeight)
	} else {
		q.verifyCh = ch
	}

	return ch
}

// Retry is called when a dispatcher failed to fetch a light block or the
// fetched light block failed verification. It signals to the queue to add the
// height back to the request queue
func (q *blockQueue) retry(height int64) {
	q.mtx.Lock()
	defer q.mtx.Unlock()

	select {
	case <-q.doneCh:
		return
	default:
	}

	// we don't need to retry if this is below the terminal height
	if q.terminal != nil && height < q.terminal.Height {
		return
	}

	q.retries++
	if q.retries >= q.maxRetries {
		q._closeChannels()
		return
	}

	if len(q.waiters) > 0 {
		q.waiters[0] <- height
		close(q.waiters[0])
		q.waiters = q.waiters[1:]
	} else {
		q.failed = insertSorted(q.failed, height)
	}
}

// Success is called when a light block has been successfully verified and
// processed
func (q *blockQueue) success(height int64) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	if q.terminal != nil && q.verifyHeight == q.terminal.Height {
		q._closeChannels()
	}
	q.verifyHeight--
}

func (q *blockQueue) error() error {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	if q.retries >= q.maxRetries {
		return fmt.Errorf("failed to backfill blocks following reverse sync. Max retries exceeded (%d). "+
			"Target height: %d, height reached: %d", q.maxRetries, q.stopHeight, q.verifyHeight)
	}
	return nil
}

// close the queue and respective channels
func (q *blockQueue) close() {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	q._closeChannels()
}

// CONTRACT: must have a write lock. Use close instead
func (q *blockQueue) _closeChannels() {
	close(q.doneCh)

	// wait for the channel to be drained
	select {
	case <-q.doneCh:
		return
	default:
	}

	for _, ch := range q.waiters {
		close(ch)
	}
	if q.verifyCh != nil {
		close(q.verifyCh)
	}
}

// nextFailed pops the next height in the list of failed heights
// CONTRACT: must have a write lock
func (q *blockQueue) _nextFailed() int64 {
	height := q.failed[0]
	if len(q.failed) > 1 {
		q.failed = q.failed[1:]
	} else {
		q.failed = []int64{}
	}
	return height
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
