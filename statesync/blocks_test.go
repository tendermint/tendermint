package statesync

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/p2p"
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
	peerID, err := p2p.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	queue := newBlockQueue(startHeight, stopHeight, stopTime)
	wg := &sync.WaitGroup{}

	// asynchronously fetch blocks and add it to the queue
	for i := 0; i <= numWorkers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case <-queue.Done():
					wg.Done()
					return
				case height := <-queue.NextHeight():
					queue.Add(mocklb(peerID, height, endTime))
				}
			}
		}()
	}

	trackingHeight := startHeight
	wg.Add(1)
	loop:for {
		select {
		case <-queue.Done():
			wg.Done()
			break loop
			
		case resp := <- queue.VerifyNext():
			// assert that the queue serializes the blocks
			assert.Equal(t, resp.block.Height, trackingHeight)
			trackingHeight--
			queue.Success(resp.block.Height)
		}
			
	}

	wg.Wait()
	assert.Less(t, trackingHeight, stopHeight)

	select {
	case <-queue.doneCh:
	default:
		t.Fatal("queue's done channel is not closed")
	}

}

// Test with spurious failures and retries
func TestBlockQueueWithFailures(t *testing.T) {
	peerID, err := p2p.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	queue := newBlockQueue(startHeight, stopHeight, stopTime)
	wg := &sync.WaitGroup{}

	failureRate := 4
	for i := 0; i <= numWorkers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case height := <-queue.NextHeight():
					if rand.Intn(failureRate) == 0 {
						queue.Retry(height)
					} else {
						queue.Add(mocklb(peerID, height, endTime))
					}
				case <-queue.Done():
					wg.Done()
					return
				}
			}
		}()
	}	

	trackingHeight := startHeight
	for {
		select {
		case resp := <- queue.VerifyNext():
			// assert that the queue serializes the blocks
			assert.Equal(t, resp.block.Height, trackingHeight)
			if rand.Intn(failureRate) == 0 {
				queue.Retry(resp.block.Height)
			} else {
				trackingHeight--
				queue.Success(resp.block.Height)
			}

		case <-queue.Done():
			wg.Wait()
			assert.Less(t, trackingHeight, stopHeight)
			return
		}
	}
}

// Test a scenario where more blocks are needed then just the stopheight because
// we haven't found a block with a small enough time.
func TestBlockQueueStopTime(t *testing.T) {
	peerID, err := p2p.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	queue := newBlockQueue(startHeight, stopHeight, stopTime)
	wg := &sync.WaitGroup{}

	baseTime := stopTime.Add(-50 * time.Second)

	// asynchronously fetch blocks and add it to the queue
	for i := 0; i <= numWorkers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case height := <-queue.NextHeight():
					t := baseTime.Add(time.Duration(height) * time.Second)
					queue.Add(mocklb(peerID, height, t))
				case <-queue.Done():
					wg.Done()
					return
				}
			}
		}()
	}

	trackingHeight := startHeight
	for {
		select {
		case resp := <- queue.VerifyNext():
			// assert that the queue serializes the blocks
			assert.Equal(t, resp.block.Height, trackingHeight)
			trackingHeight--
			queue.Success(resp.block.Height)

		case <-queue.Done():
			wg.Wait()
			assert.Less(t, trackingHeight, stopHeight - 50)
			return
		}
	}	
}

func mocklb(peer p2p.NodeID, height int64, time time.Time) lightBlockResponse {
	return lightBlockResponse{
		block: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: &types.Header{
					Time:   time,
					Height: height,
				},
				Commit: nil,
			},
			ValidatorSet: nil,
		},
		peer: peer,
	}
}
