package blockchain

import (
	"testing"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

func init() {
	minimumTimeout = 2 * time.Second
}

type testPeer struct {
	id        p2p.ID
	height    int64
	inputChan chan inputData //make sure each peer's data is sequential
}

type inputData struct {
	t       *testing.T
	pool    *BlockPool
	request BlockRequest
}

func (p testPeer) runInputRoutine(f InputFunc) {
	go func() {
		for input := range p.inputChan {
			f(input)
		}
	}()
}

type InputFunc func(input inputData)

// Request desired, pretend like we got the block immediately.
func simulateInput(input inputData) {
	block := &types.Block{Header: types.Header{Height: input.request.Height}}
	input.pool.AddBlockHeader(input.request.PeerID, input.request.Height, 123)
	input.pool.AddBlock(input.request.PeerID, block, 123)
	input.t.Logf("Added block from peer %v (height: %v)", input.request.PeerID, input.request.Height)
}

type testPeers map[p2p.ID]testPeer

func (ps testPeers) start(f InputFunc) {
	for _, v := range ps {
		v.runInputRoutine(f)
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
		peerID := p2p.ID(cmn.RandStr(12))
		height := minHeight + cmn.RandInt63n(maxHeight-minHeight)
		peers[peerID] = testPeer{peerID, height, make(chan inputData, 10)}
	}
	return peers
}

func TestBasic(t *testing.T) {
	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(start, requestsCh, errorsCh)
	pool.SetLogger(log.TestingLogger())

	err := pool.Start()
	if err != nil {
		t.Error(err)
	}

	defer pool.Stop()

	peers.start(simulateInput)
	defer peers.stop()

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerHeight(peer.id, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second := pool.PeekTwoBlocks()
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
			t.Logf("Pulled new BlockRequest %v", request)
			if request.Height == 300 {
				return // Done!
			}

			peers[request.PeerID].inputChan <- inputData{t, pool, request}
		}
	}
}

func simulateInvalidInput(input inputData) {
	val := cmn.RandInt63n(4)
	switch val {
	case 0:
		input.pool.AddBlockHeader("invalid_id", input.request.Height, 123)
	case 1:
		input.pool.AddBlockHeader(input.request.PeerID, input.request.Height, 1024*1024*1024)
		input.pool.AddBlockHeader(input.request.PeerID, input.request.Height, -1)
	case 2:
		block := &types.Block{Header: types.Header{Height: input.request.Height}}
		input.pool.AddBlockHeader(input.request.PeerID, input.request.Height, 123)
		input.pool.AddBlock(input.request.PeerID, block, 124)
	case 3:
		block := &types.Block{Header: types.Header{Height: 10000}}
		input.pool.AddBlockHeader(input.request.PeerID, 10000, 123)
		input.pool.AddBlock(input.request.PeerID, block, 123)
	}

	input.t.Logf("Added block from peer %v (height: %v)", input.request.PeerID, input.request.Height)
}

func TestInvalidInput(t *testing.T) {
	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(start, requestsCh, errorsCh)
	pool.SetLogger(log.TestingLogger())

	err := pool.Start()
	if err != nil {
		t.Error(err)
	}

	defer pool.Stop()

	peers.start(simulateInvalidInput)
	defer peers.stop()

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerHeight(peer.id, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				panic("must not reach here.")
			} else {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	var peerErrors = make(map[p2p.ID]string, 10)
	// Pull from channels
	for {
		select {
		case err := <-errorsCh:
			peerErrors[err.peerID] = err.err.Error()
			if len(peerErrors) == 10 {
				goto quit // Done!
			}
		case request := <-requestsCh:
			peers[request.PeerID].inputChan <- inputData{t, pool, request}
		}
	}

quit:
	t.Logf("pool status %v", pool.debug())
}

func TestTimeout(t *testing.T) {
	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(start, requestsCh, errorsCh)
	pool.SetLogger(log.TestingLogger())
	err := pool.Start()
	if err != nil {
		t.Error(err)
	}
	defer pool.Stop()

	for _, peer := range peers {
		t.Logf("Peer %v", peer.id)
	}

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerHeight(peer.id, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				pool.PopRequest()
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Pull from channels
	counter := 0
	timedOut := map[p2p.ID]struct{}{}
	for {
		select {
		case err := <-errorsCh:
			t.Log(err)
			// consider error to be always timeout here
			if _, ok := timedOut[err.peerID]; !ok {
				counter++
				if counter == len(peers) {
					return // Done!
				}
			}
		case request := <-requestsCh:
			t.Logf("Pulled new BlockRequest %+v", request)
		}
	}
}
