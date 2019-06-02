package blockchain

import (
	"fmt"
	"sort"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type blockPool struct {
	logger log.Logger
	// Set of peers that have sent status responses, with height bigger than pool.height
	peers map[p2p.ID]*bpPeer
	// Set of block heights and the corresponding peers from where a block response is expected or has been received.
	blocks map[int64]p2p.ID

	plannedRequests   map[int64]struct{} // list of blocks to be assigned peers for blockRequest
	nextRequestHeight int64              // next height to be added to plannedRequests

	height        int64 // height of next block to execute
	maxPeerHeight int64 // maximum height of all peers
	toBcR         bcReactor
}

func NewBlockPool(height int64, toBcR bcReactor) *blockPool {
	return &blockPool{
		peers:             make(map[p2p.ID]*bpPeer),
		maxPeerHeight:     0,
		blocks:            make(map[int64]p2p.ID),
		plannedRequests:   make(map[int64]struct{}),
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

func (pool *blockPool) SetLogger(l log.Logger) {
	pool.logger = l
}

func (pool *blockPool) ReachedMaxHeight() bool {
	return pool.height >= pool.maxPeerHeight
}

func (pool *blockPool) rescheduleRequest(peerID p2p.ID, height int64) {
	pool.logger.Info("reschedule requests made to peer for height ", "peerID", peerID, "height", height)
	pool.plannedRequests[height] = struct{}{}
	delete(pool.blocks, height)
	pool.peers[peerID].RemoveBlockAtHeight(height)
}

// Updates the pool's max height. If no peers are left maxPeerHeight is set to 0.
func (pool *blockPool) updateMaxPeerHeight() {
	var newMax int64
	for _, peer := range pool.peers {
		peerHeight := peer.GetHeight()
		if peerHeight > newMax {
			newMax = peerHeight
		}
	}
	pool.maxPeerHeight = newMax
}

// Adds a new peer or updates an existing peer with a new height.
// If a new peer is too short it is not added.
func (pool *blockPool) UpdatePeer(peerID p2p.ID, height int64) error {
	peer := pool.peers[peerID]
	oldHeight := int64(0)
	if peer != nil {
		oldHeight = peer.GetHeight()
	}
	pool.logger.Info("updatePeer", "peerID", peerID, "height", height, "old_height", oldHeight)

	if peer == nil {
		if height < pool.height {
			pool.logger.Info("Peer height too small", "peer", peerID, "height", height, "fsm_height", pool.height)
			return errPeerTooShort
		}
		// Add new peer.
		peer = NewBPPeer(peerID, height, pool.toBcR.sendPeerError, nil)
		peer.SetLogger(pool.logger.With("peer", peerID))
		pool.peers[peerID] = peer
	} else {
		// Check if peer is lowering its height. This is not allowed.
		if height < peer.GetHeight() {
			pool.RemovePeer(peerID, errPeerLowersItsHeight)
			return errPeerLowersItsHeight
		}
		// Update existing peer.
		peer.SetHeight(height)
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
	peer.StopBlockResponseTimer()
	delete(pool.peers, peer.ID())

	if peer.GetHeight() == pool.maxPeerHeight {
		pool.updateMaxPeerHeight()
	}
}

// Removes any blocks and requests associated with the peer and deletes the peer.
// Also triggers new requests if blocks have been removed.
func (pool *blockPool) RemovePeer(peerID p2p.ID, err error) {
	peer := pool.peers[peerID]
	if peer == nil {
		return
	}
	pool.logger.Info("removing peer", "peerID", peerID, "error", err)

	// Reschedule the block requests made to the peer, or received and not processed yet.
	// Note that some of the requests may be removed further down.
	for h := range pool.peers[peerID].blocks {
		pool.rescheduleRequest(peerID, h)
	}

	oldMaxPeerHeight := pool.maxPeerHeight
	// Delete the peer. This operation may result in the pool's maxPeerHeight being lowered.
	pool.deletePeer(peer)

	// Check if the pool's maxPeerHeight has been lowered.
	// This may happen if the tallest peer has been removed.
	if oldMaxPeerHeight > pool.maxPeerHeight {
		// Remove any planned requests for heights over the new maxPeerHeight.
		for h := range pool.plannedRequests {
			if h > pool.maxPeerHeight {
				delete(pool.plannedRequests, h)
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
		if peer.GetHeight() < pool.height {
			pool.RemovePeer(peer.ID(), nil)
		}
	}
}

func (pool *blockPool) removeBadPeers() {
	pool.removeShortPeers()
	for _, peer := range pool.peers {
		if err := peer.IsGood(); err != nil {
			pool.RemovePeer(peer.ID(), err)
			pool.toBcR.sendPeerError(err, peer.ID())
		}
	}
}

// Make a batch of requests sorted by height. The parameter 'maxNumRequests' includes the number of block requests
// already made.
func (pool *blockPool) makeRequestBatch(maxNumRequests int32) []int {
	pool.removeBadPeers()
	// At this point pool.requests may include heights for requests to be redone due to removal of peers:
	// - peers timed out or were removed by switch
	// - FSM timed out on waiting to advance the block execution due to missing blocks at h or h+1
	// Check if more requests should be tried by subtracting the number of requests already made from the maximum allowed
	numNeeded := int(maxNumRequests) - len(pool.blocks)
	for len(pool.plannedRequests) < numNeeded {
		if pool.nextRequestHeight > pool.maxPeerHeight {
			break
		}
		pool.plannedRequests[pool.nextRequestHeight] = struct{}{}
		pool.nextRequestHeight++
	}

	heights := make([]int, 0, len(pool.plannedRequests))
	for k := range pool.plannedRequests {
		heights = append(heights, int(k))
	}
	sort.Ints(heights)
	return heights
}

func (pool *blockPool) MakeNextRequests(maxNumRequests int32) {
	heights := pool.makeRequestBatch(maxNumRequests)
	pool.logger.Info("makeNextRequests will make following requests", "number", len(heights), "heights", heights)

	for _, height := range heights {
		h := int64(height)
		if !pool.sendRequest(h) {
			// If a good peer was not found for sending the request at height h then return,
			// as it shouldn't be possible to find a peer for h+1.
			return
		}
		delete(pool.plannedRequests, h)
	}
}

func (pool *blockPool) sendRequest(height int64) bool {
	for _, peer := range pool.peers {
		if peer.GetNumPendingBlockRequests() >= int32(maxRequestsPerPeer) {
			continue
		}
		if peer.GetHeight() < height {
			continue
		}

		err := pool.toBcR.sendBlockRequest(peer.ID(), height)
		if err == errNilPeerForBlockRequest {
			// Switch does not have this peer, remove it and continue to look for another peer.
			pool.logger.Error("switch does not have peer..removing peer selected for height", "peer", peer.ID(), "height", height)
			pool.RemovePeer(peer.ID(), err)
			continue
		}

		if err == errSendQueueFull {
			pool.logger.Error("peer queue is full", "peer", peer.ID(), "height", height)
			continue
		}

		pool.logger.Info("assigned request to peer", "peer", peer.ID(), "height", height)

		pool.blocks[height] = peer.ID()
		peer.SetBlockAtHeight(height, nil)
		peer.IncrPending()

		return true
	}
	pool.logger.Error("could not find peer to send request for block at height", "height", height)
	return false
}

// Validates that the block comes from the peer it was expected from and stores it in the 'blocks' map.
func (pool *blockPool) AddBlock(peerID p2p.ID, block *types.Block, blockSize int) error {
	peer, ok := pool.peers[peerID]
	if !ok {
		pool.logger.Error("peer does not exist in the pool", "peer", peerID, "block_received", block.Height)
		return errBadDataFromPeer
	}
	b, err := pool.peers[peerID].GetBlockAtHeight(block.Height)
	if err == errMissingRequest {
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

	peer.SetBlockAtHeight(block.Height, block)
	peer.DecrPending(blockSize)
	pool.logger.Info("added new block", "height", block.Height, "from_peer", peerID,
		"total_pool_blocks", len(pool.blocks), "peer_numPendingBlockRequests", peer.GetNumPendingBlockRequests())
	return nil
}

type blockData struct {
	block *types.Block
	peer  *bpPeer
}

func (pool *blockPool) GetBlockAndPeerAtHeight(height int64) (bData *blockData, err error) {
	peerID := pool.blocks[height]
	peer := pool.peers[peerID]
	if peer == nil {
		return nil, errMissingBlock
	}

	block, err := peer.GetBlockAtHeight(height)
	if err != nil {
		return nil, err
	}

	return &blockData{peer: peer, block: block}, nil

}

func (pool *blockPool) GetNextTwoBlocks() (first, second *blockData, err error) {
	first, err = pool.GetBlockAndPeerAtHeight(pool.height)
	second, err2 := pool.GetBlockAndPeerAtHeight(pool.height + 1)
	if err == nil {
		err = err2
	}
	return
}

// Remove the peers that sent us the first two blocks, blocks are removed by RemovePeer().
func (pool *blockPool) InvalidateFirstTwoBlocks(err error) {
	first, err1 := pool.GetBlockAndPeerAtHeight(pool.height)
	second, err2 := pool.GetBlockAndPeerAtHeight(pool.height + 1)

	if err1 == nil {
		pool.RemovePeer(first.peer.ID(), err)
	}
	if err2 == nil {
		pool.RemovePeer(second.peer.ID(), err)
	}
}

func (pool *blockPool) ProcessedCurrentHeightBlock() {
	peerID, peerOk := pool.blocks[pool.height]
	if peerOk {
		pool.peers[peerID].RemoveBlockAtHeight(pool.height)
	}
	delete(pool.blocks, pool.height)
	pool.logger.Debug("processed and removed block at height", "height", pool.height)
	pool.height++
	pool.removeShortPeers()
}

// This function is called when the FSM is not able to make progress for a certain amount of time.
// This happens if the block at either pool.height or pool.height+1 has not been delivered during this time.
func (pool *blockPool) RemovePeerAtCurrentHeights(err error) {
	peerID := pool.blocks[pool.height]
	peer, ok := pool.peers[peerID]
	if ok {
		if _, err := peer.GetBlockAtHeight(pool.height); err != nil {
			pool.logger.Info("removing peer that hasn't sent block at pool.height",
				"peer", peerID, "height", pool.height)
			pool.RemovePeer(peerID, err)
			return
		}
	}
	peerID = pool.blocks[pool.height+1]
	peer, ok = pool.peers[peerID]
	if ok {
		if _, err := peer.GetBlockAtHeight(pool.height + 1); err != nil {
			pool.logger.Info("removing peer that hasn't sent block at pool.height+1",
				"peer", peerID, "height", pool.height+1)
			pool.RemovePeer(peerID, err)
			return
		}
	}
	pool.logger.Info("no peers assigned to blocks at current height or blocks already delivered",
		"height", pool.height)
}

func (pool *blockPool) Cleanup() {
	for _, peer := range pool.peers {
		peer.Cleanup()
	}
}
