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
	// Set of peers that have sent status responses, with height bigger than pool.height
	peers map[p2p.ID]*bpPeer
	// Set of block heights and the corresponding peers from where a block response is expected or has been received.
	blocks map[int64]p2p.ID

	requests          map[int64]struct{} // list of blocks to be assigned peers for blockRequest
	nextRequestHeight int64              // next height to be added to requests

	height        int64 // height of next block to execute
	maxPeerHeight int64 // maximum height of all peers
	numPending    int32 // total numPending across peers
	toBcR         bcRMessageInterface
}

func newBlockPool(height int64, toBcR bcRMessageInterface) *blockPool {
	return &blockPool{
		peers:             make(map[p2p.ID]*bpPeer),
		maxPeerHeight:     0,
		blocks:            make(map[int64]p2p.ID),
		requests:          make(map[int64]struct{}),
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

func (pool blockPool) getMaxPeerHeight() int64 {
	return pool.maxPeerHeight
}

func (pool *blockPool) reachedMaxHeight() bool {
	return pool.height >= pool.maxPeerHeight
}

func (pool *blockPool) rescheduleRequest(peerID p2p.ID, height int64) {
	pool.logger.Info("reschedule requests made to peer for height ", "peerID", peerID, "height", height)
	pool.requests[height] = struct{}{}
	delete(pool.blocks, height)
	delete(pool.peers[peerID].blocks, height)
}

// Updates the pool's max height. If no peers are left maxPeerHeight is set to 0.
func (pool *blockPool) updateMaxPeerHeight() {
	var newMax int64
	for _, peer := range pool.peers {
		if peer.height > newMax {
			newMax = peer.height
		}
	}
	pool.maxPeerHeight = newMax
}

// Adds a new peer or updates an existing peer with a new height.
// If a new peer is too short it is not added.
func (pool *blockPool) updatePeer(peerID p2p.ID, height int64) error {
	peer := pool.peers[peerID]
	oldHeight := int64(0)
	if peer != nil {
		oldHeight = peer.height
	}
	pool.logger.Debug("updatePeer", "peerID", peerID, "height", height, "old_height", oldHeight)

	if peer == nil {
		if height < pool.height {
			pool.logger.Info("Peer height too small", "peer", peerID, "height", height, "fsm_height", pool.height)
			return errPeerTooShort
		}
		// Add new peer.
		peer = newBPPeer(peerID, height, pool.toBcR.sendPeerError)
		peer.setLogger(pool.logger.With("peer", peerID))
		pool.peers[peerID] = peer
	} else {
		// Check if peer is lowering its height. This is not allowed.
		if height < peer.height {
			pool.removePeer(peerID, errPeerLowersItsHeight)
			return errPeerLowersItsHeight
		}
		// Update existing peer.
		peer.height = height
	}

	// Update the pool's maxPeerHeight if needed. Note that for updates it can only increase or left unchanged.
	pool.updateMaxPeerHeight()

	return nil
}

// Stops the peer timer and deletes the peer. Recomputes the max peer height.
func (pool *blockPool) deletePeer(peer *bpPeer) {
	if peer == nil {
		return
	}
	if peer.timeout != nil {
		peer.timeout.Stop()
	}
	delete(pool.peers, peer.id)

	if peer.height == pool.maxPeerHeight {
		pool.updateMaxPeerHeight()
	}
}

// Removes any blocks and requests associated with the peer and deletes the peer.
// Also triggers new requests if blocks have been removed.
func (pool *blockPool) removePeer(peerID p2p.ID, err error) {
	pool.logger.Info("removing peer", "peerID", peerID, "error", err)

	peer := pool.peers[peerID]
	if peer == nil {
		return
	}
	// Reschedule the block requests made to the peer, or received and not processed yet.
	// Note that some of the requests may be removed further down.
	for h, block := range pool.peers[peerID].blocks {
		if block == nil {
			pool.numPending--
		}
		pool.rescheduleRequest(peerID, h)
	}

	oldMaxPeerHeight := pool.maxPeerHeight
	// Delete the peer. This operation may result in the pool's maxPeerHeight being lowered.
	pool.deletePeer(peer)

	// Check if the pool's maxPeerHeight has been lowered.
	// This may happen if the tallest peer has been removed.
	if oldMaxPeerHeight > pool.maxPeerHeight {
		// Remove any planned requests for heights over the new maxPeerHeight.
		for h := range pool.requests {
			if h > pool.maxPeerHeight {
				delete(pool.requests, h)
			}
		}
		// Adjust the nextRequestHeight to the new max plus one.
		if pool.nextRequestHeight > pool.maxPeerHeight {
			pool.nextRequestHeight = pool.maxPeerHeight + 1
		}
	}
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
			peer.errFunc(err, peer.id)
		}
	}
}

func (pool *blockPool) makeRequestBatch(maxNumRequests int32) []int {
	pool.removeBadPeers()
	// If running low on planned requests, make more.
	numNeeded := int32(cmn.MinInt(int(maxNumRequests), len(pool.peers)*int(maxRequestsPerPeer)))
	for int32(len(pool.requests)) < numNeeded {
		if pool.nextRequestHeight > pool.maxPeerHeight {
			break
		}
		pool.requests[pool.nextRequestHeight] = struct{}{}
		pool.nextRequestHeight++
	}

	heights := make([]int, 0, len(pool.requests))
	for k := range pool.requests {
		heights = append(heights, int(k))
	}
	sort.Ints(heights)
	return heights
}

// returns the number of blocks waiting to be processed
func (pool *blockPool) getNumberOfBlocksAdded() int32 {
	// pool.blocks includes requests for which blocks have or have not been received.
	// pool.numPending is the number of pending requests (for which blocks have not been received)
	return int32(len(pool.blocks)) - pool.numPending
}

func (pool *blockPool) makeNextRequests(maxNumRequests int32) {
	heights := pool.makeRequestBatch(maxNumRequests)
	pool.logger.Info("makeNextRequests will make following requests", "number", len(heights), "heights", heights)

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

		if err := pool.toBcR.sendBlockRequest(peer.id, height); err == errNilPeerForBlockRequest {
			// Switch does not have this peer, remove it and continue to look for another peer.
			pool.logger.Info("switch does not have peer %v..removing peer selected for height", "peer", peer.id, "height", height)
			pool.removePeer(peer.id, err)
			continue
		}

		// Log with Info if the request is made for current processing height, Debug otherwise
		if height == pool.height || height == pool.height+1 {
			pool.logger.Info("assigned request to peer", "peer", peer.id, "height", height)
		} else {
			pool.logger.Debug("assigned request to peer", "peer", peer.id, "height", height)
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
	peer, ok := pool.peers[peerID]
	if !ok {
		pool.logger.Error("peer doesn't exist", "peer", peerID, "block_received", block.Height)
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

	peer.blocks[block.Height] = block
	pool.numPending--
	peer.decrPending(blockSize)
	pool.logger.Info("added new block", "height", block.Height, "from_peer", peerID, "total", len(pool.blocks))
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

// This function is called when the FSM is not able to make progress for a certain amount of time.
// This happens if the block at either pool.height or pool.height+1 has not been delivered during this time.
func (pool *blockPool) removePeerAtCurrentHeights(err error) {
	peerID := pool.blocks[pool.height]
	peer, ok := pool.peers[peerID]
	if ok && peer.blocks[pool.height] == nil {
		pool.logger.Info("removing peer %v that hasn't sent block at pool.height %v", peer.id, pool.height)
		pool.removePeer(peer.id, err)
		return
	}
	peerID = pool.blocks[pool.height+1]
	peer, ok = pool.peers[peerID]
	if ok && peer.blocks[pool.height+1] == nil {
		pool.logger.Info("removing peer %v that hasn't sent block at pool.height+1 %v", peer.id, pool.height+1)
		pool.removePeer(peer.id, err)
		return
	}
	pool.logger.Info("no peers assigned to blocks at current height or blocks already delivered",
		"height", pool.height)
}

func (pool *blockPool) cleanup() {
	for _, peer := range pool.peers {
		peer.cleanup()
	}
}
