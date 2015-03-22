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
	timeoutsCh := make(chan string)
	requestsCh := make(chan BlockRequest)
	blocksCh := make(chan *Block)

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
			log.Debug("Pulled new BlockRequest", "request", request)
			// After a while, pretend like we got a block from the peer.
			go func() {
				block := &Block{Header: &Header{Height: request.Height}}
				pool.AddBlock(block, request.PeerId)
				log.Debug("Added block", "block", request.Height, "peer", request.PeerId)
			}()
		case block := <-blocksCh:
			log.Debug("Pulled new Block", "height", block.Height)
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
