package statesync

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/types"
)

var (
	startHeight int64 = 200
	stopHeight  int64 = 100
	stopTime          = time.Date(2019, 1, 1, 1, 0, 0, 0, time.UTC)
	endTime           = stopTime.Add(-1 * time.Second)
	numWorkers        = 1
)

func TestBlockQueueBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	queue := newBlockQueue(startHeight, stopHeight, 1, stopTime, 1)
	wg := &sync.WaitGroup{}

	// asynchronously fetch blocks and add it to the queue
	for i := 0; i <= numWorkers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case height := <-queue.nextHeight():
					queue.add(mockLBResp(ctx, t, peerID, height, endTime))
				case <-queue.done():
					wg.Done()
					return
				}
			}
		}()
	}

	trackingHeight := startHeight
	wg.Add(1)

loop:
	for {
		select {
		case <-queue.done():
			wg.Done()
			break loop

		case resp := <-queue.verifyNext():
			// assert that the queue serializes the blocks
			require.Equal(t, resp.block.Height, trackingHeight)
			trackingHeight--
			queue.success()
		}

	}

	wg.Wait()
	assert.Less(t, trackingHeight, stopHeight)
}

// Test with spurious failures and retries
func TestBlockQueueWithFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	queue := newBlockQueue(startHeight, stopHeight, 1, stopTime, 200)
	wg := &sync.WaitGroup{}

	failureRate := 4
	for i := 0; i <= numWorkers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case height := <-queue.nextHeight():
					if rand.Intn(failureRate) == 0 {
						queue.retry(height)
					} else {
						queue.add(mockLBResp(ctx, t, peerID, height, endTime))
					}
				case <-queue.done():
					wg.Done()
					return
				}
			}
		}()
	}

	trackingHeight := startHeight
	for {
		select {
		case resp := <-queue.verifyNext():
			// assert that the queue serializes the blocks
			assert.Equal(t, resp.block.Height, trackingHeight)
			if rand.Intn(failureRate) == 0 {
				queue.retry(resp.block.Height)
			} else {
				trackingHeight--
				queue.success()
			}

		case <-queue.done():
			wg.Wait()
			assert.Less(t, trackingHeight, stopHeight)
			return
		}
	}
}

// Test that when all the blocks are retrieved that the queue still holds on to
// it's workers and in the event of failure can still fetch the failed block
func TestBlockQueueBlocks(t *testing.T) {
	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)
	queue := newBlockQueue(startHeight, stopHeight, 1, stopTime, 2)
	expectedHeight := startHeight
	retryHeight := stopHeight + 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

loop:
	for {
		select {
		case height := <-queue.nextHeight():
			require.Equal(t, height, expectedHeight)
			require.GreaterOrEqual(t, height, stopHeight)
			expectedHeight--
			queue.add(mockLBResp(ctx, t, peerID, height, endTime))
		case <-time.After(1 * time.Second):
			if expectedHeight >= stopHeight {
				t.Fatalf("expected next height %d", expectedHeight)
			}
			break loop
		}
	}

	// close any waiter channels that the previous worker left hanging
	for _, ch := range queue.waiters {
		close(ch)
	}
	queue.waiters = make([]chan int64, 0)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	// so far so good. The worker is waiting. Now we fail a previous
	// block and check that the worker fetches them
	go func(t *testing.T) {
		defer wg.Done()
		select {
		case height := <-queue.nextHeight():
			require.Equal(t, retryHeight, height)
		case <-time.After(1 * time.Second):
			require.Fail(t, "queue didn't ask worker to fetch failed height")
		}
	}(t)
	queue.retry(retryHeight)
	wg.Wait()

}

func TestBlockQueueAcceptsNoMoreBlocks(t *testing.T) {
	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)
	queue := newBlockQueue(startHeight, stopHeight, 1, stopTime, 1)
	defer queue.close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

loop:
	for {
		select {
		case height := <-queue.nextHeight():
			require.GreaterOrEqual(t, height, stopHeight)
			queue.add(mockLBResp(ctx, t, peerID, height, endTime))
		case <-time.After(1 * time.Second):
			break loop
		}
	}

	require.Len(t, queue.pending, int(startHeight-stopHeight)+1)

	queue.add(mockLBResp(ctx, t, peerID, stopHeight-1, endTime))
	require.Len(t, queue.pending, int(startHeight-stopHeight)+1)
}

// Test a scenario where more blocks are needed then just the stopheight because
// we haven't found a block with a small enough time.
func TestBlockQueueStopTime(t *testing.T) {
	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	queue := newBlockQueue(startHeight, stopHeight, 1, stopTime, 1)
	wg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseTime := stopTime.Add(-50 * time.Second)

	// asynchronously fetch blocks and add it to the queue
	for i := 0; i <= numWorkers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case height := <-queue.nextHeight():
					blockTime := baseTime.Add(time.Duration(height) * time.Second)
					queue.add(mockLBResp(ctx, t, peerID, height, blockTime))
				case <-queue.done():
					wg.Done()
					return
				}
			}
		}()
	}

	trackingHeight := startHeight
	for {
		select {
		case resp := <-queue.verifyNext():
			// assert that the queue serializes the blocks
			assert.Equal(t, resp.block.Height, trackingHeight)
			trackingHeight--
			queue.success()

		case <-queue.done():
			wg.Wait()
			assert.Less(t, trackingHeight, stopHeight-50)
			return
		}
	}
}

func TestBlockQueueInitialHeight(t *testing.T) {
	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)
	const initialHeight int64 = 120

	queue := newBlockQueue(startHeight, stopHeight, initialHeight, stopTime, 1)
	wg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// asynchronously fetch blocks and add it to the queue
	for i := 0; i <= numWorkers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case height := <-queue.nextHeight():
					require.GreaterOrEqual(t, height, initialHeight)
					queue.add(mockLBResp(ctx, t, peerID, height, endTime))
				case <-queue.done():
					wg.Done()
					return
				}
			}
		}()
	}

loop:
	for {
		select {
		case <-queue.done():
			wg.Wait()
			require.NoError(t, queue.error())
			break loop

		case resp := <-queue.verifyNext():
			require.GreaterOrEqual(t, resp.block.Height, initialHeight)
			queue.success()
		}
	}
}

func mockLBResp(ctx context.Context, t *testing.T, peer types.NodeID, height int64, time time.Time) lightBlockResponse {
	t.Helper()
	vals, pv := factory.ValidatorSet(ctx, t, 3, 10)
	_, _, lb := mockLB(ctx, t, height, time, factory.MakeBlockID(), vals, pv)
	return lightBlockResponse{
		block: lb,
		peer:  peer,
	}
}
