package blocksync

import (
	"context"
	"fmt"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
)

func init() {
	peerTimeout = 2 * time.Second
}

type testPeer struct {
	id        types.NodeID
	base      int64
	height    int64
	inputChan chan inputData // make sure each peer's data is sequential
}

type inputData struct {
	t       *testing.T
	pool    *BlockPool
	request BlockRequest
}

func (p testPeer) runInputRoutine() {
	go func() {
		for input := range p.inputChan {
			p.simulateInput(input)
		}
	}()
}

// Request desired, pretend like we got the block immediately.
func (p testPeer) simulateInput(input inputData) {
	block := &types.Block{Header: types.Header{Height: input.request.Height}}
	extCommit := &types.ExtendedCommit{
		Height: input.request.Height,
	}
	_ = input.pool.AddBlock(input.request.PeerID, block, extCommit, 123)
	// TODO: uncommenting this creates a race which is detected by:
	// https://github.com/golang/go/blob/2bd767b1022dd3254bcec469f0ee164024726486/src/testing/testing.go#L854-L856
	// see: https://github.com/tendermint/tendermint/issues/3390#issue-418379890
	// input.t.Logf("Added block from peer %v (height: %v)", input.request.PeerID, input.request.Height)
}

type testPeers map[types.NodeID]testPeer

func (ps testPeers) start() {
	for _, v := range ps {
		v.runInputRoutine()
	}
}

func (ps testPeers) stop() {
	for _, v := range ps {
		close(v.inputChan)
	}
}

func makePeers(numPeers int, minHeight, maxHeight int64) testPeers {
	peers := make(testPeers, numPeers)
	for i := 0; i < numPeers; i++ {
		peerID := types.NodeID(tmrand.Str(12))
		height := minHeight + mrand.Int63n(maxHeight-minHeight)
		base := minHeight + int64(i)
		if base > height {
			base = height
		}
		peers[peerID] = testPeer{peerID, base, height, make(chan inputData, 10)}
	}
	return peers
}

func TestBlockPoolBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(log.NewNopLogger(), start, requestsCh, errorsCh)

	if err := pool.Start(ctx); err != nil {
		t.Error(err)
	}

	t.Cleanup(func() { cancel(); pool.Wait() })

	peers.start()
	defer peers.stop()

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerRange(peer.id, peer.base, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second, _ := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				pool.PopRequest()
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Pull from channels
	for {
		select {
		case err := <-errorsCh:
			t.Error(err)
		case request := <-requestsCh:
			if request.Height == 300 {
				return // Done!
			}

			peers[request.PeerID].inputChan <- inputData{t, pool, request}
		}
	}
}

func TestBlockPoolTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(logger, start, requestsCh, errorsCh)
	err := pool.Start(ctx)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(func() { cancel(); pool.Wait() })

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerRange(peer.id, peer.base, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second, _ := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				pool.PopRequest()
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Pull from channels
	counter := 0
	timedOut := map[types.NodeID]struct{}{}
	for {
		select {
		case err := <-errorsCh:
			// consider error to be always timeout here
			if _, ok := timedOut[err.peerID]; !ok {
				counter++
				if counter == len(peers) {
					return // Done!
				}
			}
		case request := <-requestsCh:
			logger.Debug("received request",
				"counter", counter,
				"request", request)
		}
	}
}

func TestBlockPoolRemovePeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peers := make(testPeers, 10)
	for i := 0; i < 10; i++ {
		peerID := types.NodeID(fmt.Sprintf("%d", i+1))
		height := int64(i + 1)
		peers[peerID] = testPeer{peerID, 0, height, make(chan inputData)}
	}
	requestsCh := make(chan BlockRequest)
	errorsCh := make(chan peerError)

	pool := NewBlockPool(log.NewNopLogger(), 1, requestsCh, errorsCh)
	err := pool.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { cancel(); pool.Wait() })

	// add peers
	for peerID, peer := range peers {
		pool.SetPeerRange(peerID, peer.base, peer.height)
	}
	assert.EqualValues(t, 10, pool.MaxPeerHeight())

	// remove not-existing peer
	assert.NotPanics(t, func() { pool.RemovePeer(types.NodeID("Superman")) })

	// remove peer with biggest height
	pool.RemovePeer(types.NodeID("10"))
	assert.EqualValues(t, 9, pool.MaxPeerHeight())

	// remove all peers
	for peerID := range peers {
		pool.RemovePeer(peerID)
	}

	assert.EqualValues(t, 0, pool.MaxPeerHeight())
}
