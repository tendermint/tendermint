package block

import (
	"math/rand"
	"testing"

	. "github.com/tendermint/tendermint/common"
)

type testPeer struct {
	id     string
	height uint
}

func makePeers(numPeers int, minHeight, maxHeight uint) map[string]testPeer {
	peers := make(map[string]testPeer, numPeers)
	for i := 0; i < numPeers; i++ {
		peerId := RandStr(12)
		height := minHeight + uint(rand.Intn(int(maxHeight-minHeight)))
		peers[peerId] = testPeer{peerId, height}
	}
	return peers
}

func TestBasic(t *testing.T) {
	// 100 peers anywhere at height 0 to 1000.
	peers := makePeers(100, 0, 1000)

	start := uint(42)
	maxHeight := uint(300)
	timeoutsCh := make(chan string, 100)
	requestsCh := make(chan BlockRequest, 100)
	blocksCh := make(chan *Block, 100)

	pool := NewBlockPool(start, timeoutsCh, requestsCh, blocksCh)
	pool.Start()

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerStatus(peer.id, peer.height)
		}
	}()

	lastSeenBlock := uint(41)

	// Pull from channels
	for {
		select {
		case peerId := <-timeoutsCh:
			t.Errorf("timeout: %v", peerId)
		case request := <-requestsCh:
			log.Debug("TEST: Pulled new BlockRequest", "request", request)
			// After a while, pretend like we got a block from the peer.
			go func() {
				block := &Block{Header: &Header{Height: request.Height}}
				pool.AddBlock(block, request.PeerId)
				log.Debug("TEST: Added block", "block", request.Height, "peer", request.PeerId)
			}()
		case block := <-blocksCh:
			log.Debug("TEST: Pulled new Block", "height", block.Height)
			if block.Height != lastSeenBlock+1 {
				t.Fatalf("Wrong order of blocks seen. Expected: %v Got: %v", lastSeenBlock+1, block.Height)
			}
			lastSeenBlock++
			if block.Height == maxHeight {
				return // Done!
			}
		}
	}

	pool.Stop()
}

func TestTimeout(t *testing.T) {
	peers := makePeers(100, 0, 1000)
	start := uint(42)
	timeoutsCh := make(chan string, 10)
	requestsCh := make(chan BlockRequest, 10)
	blocksCh := make(chan *Block, 100)

	pool := NewBlockPool(start, timeoutsCh, requestsCh, blocksCh)
	pool.Start()

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerStatus(peer.id, peer.height)
		}
	}()

	// Pull from channels
	for {
		select {
		case peerId := <-timeoutsCh:
			// Timed out. Done!
			if peers[peerId].id != peerId {
				t.Errorf("Unexpected peer from timeoutsCh")
			}
			//return
		case _ = <-requestsCh:
			// Don't do anything, let it time out.
		case _ = <-blocksCh:
			t.Errorf("Got block when none expected")
			return
		}
	}

	pool.Stop()

}
