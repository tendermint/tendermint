package blockchain

import (
	"math/rand"
	"testing"
	"time"

	"github.com/tendermint/tendermint/types"
	. "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

func init() {
	peerTimeoutSeconds = time.Duration(2)
}

type testPeer struct {
	id     string
	height int
}

func makePeers(numPeers int, minHeight, maxHeight int) map[string]testPeer {
	peers := make(map[string]testPeer, numPeers)
	for i := 0; i < numPeers; i++ {
		peerID := RandStr(12)
		height := minHeight + rand.Intn(maxHeight-minHeight)
		peers[peerID] = testPeer{peerID, height}
	}
	return peers
}

func TestBasic(t *testing.T) {
	start := 42
	peers := makePeers(10, start+1, 1000)
	timeoutsCh := make(chan string, 100)
	requestsCh := make(chan BlockRequest, 100)
	pool := NewBlockPool(start, requestsCh, timeoutsCh)
	pool.SetLogger(log.TestingLogger())
	pool.Start()
	defer pool.Stop()

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
		case peerID := <-timeoutsCh:
			t.Errorf("timeout: %v", peerID)
		case request := <-requestsCh:
			t.Logf("Pulled new BlockRequest %v", request)
			if request.Height == 300 {
				return // Done!
			}
			// Request desired, pretend like we got the block immediately.
			go func() {
				block := &types.Block{Header: &types.Header{Height: request.Height}}
				pool.AddBlock(request.PeerID, block, 123)
				t.Logf("Added block from peer %v (height: %v)", request.PeerID, request.Height)
			}()
		}
	}
}

func TestTimeout(t *testing.T) {
	start := 42
	peers := makePeers(10, start+1, 1000)
	timeoutsCh := make(chan string, 100)
	requestsCh := make(chan BlockRequest, 100)
	pool := NewBlockPool(start, requestsCh, timeoutsCh)
	pool.SetLogger(log.TestingLogger())
	pool.Start()
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
	timedOut := map[string]struct{}{}
	for {
		select {
		case peerID := <-timeoutsCh:
			t.Logf("Peer %v timeouted", peerID)
			if _, ok := timedOut[peerID]; !ok {
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
