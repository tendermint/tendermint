package blockchain

import (
	"fmt"
	"sort"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type blockData struct {
	block *types.Block
	peer  *bpPeer
}

func (bd *blockData) String() string {
	if bd == nil {
		return fmt.Sprintf("blockData nil")
	}
	if bd.block == nil {
		if bd.peer == nil {
			return fmt.Sprintf("block: nil peer: nil")
		}
		return fmt.Sprintf("block: nil peer: %v", bd.peer.id)
	}
	return fmt.Sprintf("block: %v peer: %v", bd.block.Height, bd.peer.id)
}

type blockPool struct {
	logger log.Logger
	peers  map[p2p.ID]*bpPeer
	blocks map[int64]p2p.ID

	requests          map[int64]bool // list of blocks to be assigned peers for blockRequest
	nextRequestHeight int64          // next height to be added to requests

	height        int64 // processing height
	maxPeerHeight int64 // maximum height of all peers
	numPending    int32 // total numPending across peers
	toBcR         bcRMessageInterface
}

func newBlockPool(height int64, toBcR bcRMessageInterface) *blockPool {
	return &blockPool{
		peers:             make(map[p2p.ID]*bpPeer),
		maxPeerHeight:     0,
		blocks:            make(map[int64]p2p.ID),
		requests:          make(map[int64]bool),
		nextRequestHeight: height,
		height:            height,
		toBcR:             toBcR,
	}
}

func (pool *blockPool) String() string {
	peerStr := fmt.Sprintf("Pool Peers:")
	for _, p := range pool.peers {
		peerStr += fmt.Sprintf("%v,", p)
	}
	return peerStr
}

func (pool *blockPool) setLogger(l log.Logger) {
	pool.logger = l
}

// GetStatus returns pool's height, numPending requests and the number of
// requests ready to be send in the future.
func (pool *blockPool) getStatus() (height int64, numPending int32, maxPeerHeight int64) {
	return pool.height, pool.numPending, pool.maxPeerHeight
}

func (pool blockPool) getMaxPeerHeight() int64 {
	return pool.maxPeerHeight
}

func (pool *blockPool) reachedMaxHeight() bool {
	return pool.maxPeerHeight == 0 || pool.height >= pool.maxPeerHeight
}

func (pool *blockPool) rescheduleRequest(peerID p2p.ID, height int64) {
	pool.logger.Debug("reschedule requests made to peer for height ", "peerID", peerID, "height", height)
	pool.requests[height] = true
	delete(pool.blocks, height)
	delete(pool.peers[peerID].blocks, height)
}

// Updates the pool's max height. If no peers are left maxPeerHeight is set to 0.
func (pool *blockPool) updateMaxPeerHeight() {
	var max int64
	for _, peer := range pool.peers {
		if peer.height > max {
			max = peer.height
		}
	}
	pool.maxPeerHeight = max
}

// Adds a new peer or updates an existing peer with a new height.
// If the peer is too short it is removed.
func (pool *blockPool) updatePeer(peerID p2p.ID, height int64) error {
	pool.logger.Debug("updatePeer", "peerID", peerID, "height", height)
	peer := pool.peers[peerID]

	if height < pool.height {
		pool.logger.Info("Peer height too small", "peer", peerID, "height", height, "fsm_height", pool.height)

		// Don't add or update a peer that is not useful.
		if peer != nil {
			pool.logger.Info("remove short peer", "peer", peerID, "height", height, "fsm_height", pool.height)
			pool.removePeer(peerID, errPeerTooShort)
		}
		return errPeerTooShort
	}

	if peer == nil {
		// Add new peer.
		peer = newBPPeer(peerID, height, pool.toBcR.sendPeerError)
		peer.setLogger(pool.logger.With("peer", peerID))
		pool.peers[peerID] = peer
	} else {
		// Update existing peer.
		// Remove any requests made for heights in (height, peer.height].
		for h, block := range pool.peers[peerID].blocks {
			if h <= height {
				continue
			}
			// Reschedule the requests for all blocks waiting for the peer, or received and not processed yet.
			if block == nil {
				// Since block was not yet received it is counted in numPending, decrement.
				pool.numPending--
				pool.peers[peerID].numPending--
			}
			pool.rescheduleRequest(peerID, h)
		}
		peer.height = height
	}

	pool.updateMaxPeerHeight()

	return nil
}

// Stops the peer timer and deletes the peer. Recomputes the max peer height.
func (pool *blockPool) deletePeer(peerID p2p.ID) {
	if p, ok := pool.peers[peerID]; ok {
		if p.timeout != nil {
			p.timeout.Stop()
		}
		delete(pool.peers, peerID)

		if p.height == pool.maxPeerHeight {
			pool.updateMaxPeerHeight()
		}
	}
}

// Removes any blocks and requests associated with the peer and deletes the peer.
// Also triggers new requests if blocks have been removed.
func (pool *blockPool) removePeer(peerID p2p.ID, err error) {
	pool.logger.Debug("removing peer", "peerID", peerID)

	peer := pool.peers[peerID]
	if peer == nil {
		return
	}
	// Reschedule the requests for all blocks waiting for the peer, or received and not processed yet.
	for h, block := range pool.peers[peerID].blocks {
		if block == nil {
			pool.numPending--
		}
		pool.rescheduleRequest(peerID, h)
	}
	pool.deletePeer(peerID)

}

// Called every time FSM advances its height.
func (pool *blockPool) removeShortPeers() {
	for _, peer := range pool.peers {
		if peer.height < pool.height {
			pool.removePeer(peer.id, nil)
		}
	}
}

func (pool *blockPool) removeBadPeers() {
	pool.removeShortPeers()
	for _, peer := range pool.peers {
		if err := peer.isGood(); err != nil {
			pool.removePeer(peer.id, err)
			if err == errSlowPeer {
				peer.errFunc(errSlowPeer, peer.id)
			}
		}
	}
}

func (pool *blockPool) makeRequestBatch(maxNumPendingRequests int32) []int {
	pool.removeBadPeers()
	// If running low on planned requests, make more.
	numNeeded := int32(cmn.MinInt(int(maxNumPendingRequests), len(pool.peers)*int(maxRequestsPerPeer))) - pool.numPending
	for int32(len(pool.requests)) < numNeeded {
		if pool.nextRequestHeight > pool.maxPeerHeight {
			break
		}
		pool.requests[pool.nextRequestHeight] = true
		pool.nextRequestHeight++
	}

	heights := make([]int, 0, len(pool.requests))
	for k := range pool.requests {
		heights = append(heights, int(k))
	}
	sort.Ints(heights)
	return heights
}

func (pool *blockPool) makeNextRequests(maxNumPendingRequests int32) {
	heights := pool.makeRequestBatch(maxNumPendingRequests)
	pool.logger.Debug("makeNextRequests will make following requests", "number", len(heights), "heights", heights)

	for _, height := range heights {
		h := int64(height)
		if !pool.sendRequest(h) {
			return
		}
		delete(pool.requests, h)
	}
}

func (pool *blockPool) sendRequest(height int64) bool {
	for _, peer := range pool.peers {
		if peer.numPending >= int32(maxRequestsPerPeer) {
			continue
		}
		if peer.height < height {
			continue
		}
		pool.logger.Debug("assign request to peer", "peer", peer.id, "height", height)
		if err := pool.toBcR.sendBlockRequest(peer.id, height); err == errNilPeerForBlockRequest {
			pool.removePeer(peer.id, err)
		}

		pool.blocks[height] = peer.id
		pool.numPending++

		peer.blocks[height] = nil
		peer.incrPending()

		return true
	}
	pool.logger.Error("could not find peer to send request for block at height", "height", height)
	return false
}

// Validates that the block comes from the peer it was expected from and stores it in the 'blocks' map.
func (pool *blockPool) addBlock(peerID p2p.ID, block *types.Block, blockSize int) error {
	if _, ok := pool.peers[peerID]; !ok {
		pool.logger.Error("peer doesn't exist", "peer", peerID, "block_receieved", block.Height)
		return errBadDataFromPeer
	}
	b, ok := pool.peers[peerID].blocks[block.Height]
	if !ok {
		pool.logger.Error("peer sent us a block we didn't expect", "peer", peerID, "blockHeight", block.Height)
		if expPeerID, pok := pool.blocks[block.Height]; pok {
			pool.logger.Error("expected this block from peer", "peer", expPeerID)
		}
		return errBadDataFromPeer
	}
	if b != nil {
		pool.logger.Error("already have a block for height", "height", block.Height)
		return errBadDataFromPeer
	}

	pool.peers[peerID].blocks[block.Height] = block
	pool.blocks[block.Height] = peerID
	pool.numPending--
	pool.peers[peerID].decrPending(blockSize)
	pool.logger.Debug("added new block", "height", block.Height, "from_peer", peerID, "total", len(pool.blocks))
	return nil
}

func (pool *blockPool) getBlockAndPeerAtHeight(height int64) (bData *blockData, err error) {
	peerID := pool.blocks[height]
	peer := pool.peers[peerID]
	if peer == nil {
		return nil, errMissingBlocks
	}

	block, ok := peer.blocks[height]
	if !ok || block == nil {
		return nil, errMissingBlocks
	}

	return &blockData{peer: peer, block: block}, nil

}

func (pool *blockPool) getNextTwoBlocks() (first, second *blockData, err error) {
	first, err = pool.getBlockAndPeerAtHeight(pool.height)
	second, err2 := pool.getBlockAndPeerAtHeight(pool.height + 1)
	if err == nil {
		err = err2
	}

	pool.logger.Debug("blocks at height and/ or height+1", "first", first, "second", second)

	return
}

// Remove the peers that sent us the first two blocks, blocks are removed by removePeer().
func (pool *blockPool) invalidateFirstTwoBlocks(err error) {
	first, err1 := pool.getBlockAndPeerAtHeight(pool.height)
	second, err2 := pool.getBlockAndPeerAtHeight(pool.height + 1)

	if err1 == nil {
		pool.removePeer(first.peer.id, err)
	}
	if err2 == nil {
		pool.removePeer(second.peer.id, err)
	}
}

func (pool *blockPool) processedCurrentHeightBlock() {
	peerID, peerOk := pool.blocks[pool.height]
	if peerOk {
		delete(pool.peers[peerID].blocks, pool.height)
	}
	delete(pool.blocks, pool.height)
	pool.logger.Debug("processed and removed block at height", "height", pool.height)
	pool.height++
	pool.removeShortPeers()
}

func (pool *blockPool) removePeerAtCurrentHeight(err error) {
	first, err := pool.getBlockAndPeerAtHeight(pool.height)
	if err == nil {
		pool.removePeer(first.peer.id, err)
	}
}

func (pool *blockPool) cleanup() {
	for _, peer := range pool.peers {
		if peer.timeout != nil {
			peer.timeout.Stop()
		}
	}
}
