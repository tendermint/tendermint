package blockchain_new

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type blockData struct {
	block  *types.Block
	peerId p2p.ID
}

func (bd *blockData) String() string {
	if bd == nil {
		return fmt.Sprintf("blockData nil")
	}
	if bd.block == nil {
		return fmt.Sprintf("block: nil peer: %v", bd.peerId)
	}
	return fmt.Sprintf("block: %v peer: %v", bd.block.Height, bd.peerId)

}

type blockPool struct {
	logger        log.Logger
	peers         map[p2p.ID]*bpPeer
	blocks        map[int64]*blockData
	height        int64 // processing height
	maxPeerHeight int64
}

func newBlockPool(height int64) *blockPool {
	return &blockPool{
		peers:         make(map[p2p.ID]*bpPeer),
		maxPeerHeight: 0,
		blocks:        make(map[int64]*blockData),
		height:        height,
	}
}

func (pool *blockPool) setLogger(l log.Logger) {
	pool.logger = l
}

// Sets the peer's blockchain height.
func (pool *blockPool) updatePeer(peerID p2p.ID, height int64, errFunc func(err error, peerID p2p.ID)) error {

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
		peer = newBPPeer(peerID, height, errFunc)
		peer.setLogger(pool.logger.With("peer", peerID))
		pool.peers[peerID] = peer
	} else {
		// remove any requests made for heights in (height, peer.height]
		for blockHeight, bData := range pool.blocks {
			if bData.peerId == peerID && blockHeight > height {
				delete(pool.blocks, blockHeight)
			}
		}
	}

	peer.height = height
	if height > pool.maxPeerHeight {
		pool.maxPeerHeight = height
	}
	return nil
}

func (pool blockPool) getMaxPeerHeight() int64 {
	return pool.maxPeerHeight
}

// called from:
// - the switch from its go routing
// - when peer times out from the timer go routine.
// Send message to FSM
func (fsm *bReactorFSM) processPeerError(err error, peerID p2p.ID) {
	msgData := bReactorMessageData{
		event: peerErrEv,
		data: bReactorEventData{
			err:    err,
			peerId: peerID,
		},
	}
	sendMessageToFSM(fsm, msgData)
}

func (pool *blockPool) reachedMaxHeight() bool {
	return pool.height >= pool.maxPeerHeight
}

// called every time FSM advances its height
func (pool *blockPool) removeShortPeers() {
	for _, peer := range pool.peers {
		if peer.height < pool.height {
			pool.logger.Info("removeShortPeers", "peer", peer.id)
			pool.removePeer(peer.id, nil)
		}
	}
}

// stops the peer timer and deletes the peer
func (pool *blockPool) deletePeer(peerID p2p.ID) {
	if p, exist := pool.peers[peerID]; exist && p.timeout != nil {
		p.timeout.Stop()
	}
	delete(pool.peers, peerID)
}

// removes any blocks and requests associated with the peer, deletes the peer and informs the switch if needed.
func (pool *blockPool) removePeer(peerID p2p.ID, err error) {
	pool.logger.Debug("removePeer", "peer", peerID, "err", err)
	// remove all data for blocks waiting for the peer or not processed yet
	for h, bData := range pool.blocks {
		if bData.peerId == peerID {
			if h == pool.height {
			}
			delete(pool.blocks, h)
		}
	}
	// delete peer
	pool.deletePeer(peerID)

	// recompute maxPeerHeight
	pool.maxPeerHeight = 0
	for _, peer := range pool.peers {
		if peer.height > pool.maxPeerHeight {
			pool.maxPeerHeight = peer.height
		}
	}
}

// Validates that the block comes from the peer it was expected from and stores it in the 'blocks' map.
func (pool *blockPool) addBlock(peerID p2p.ID, block *types.Block, blockSize int) error {

	blockData := pool.blocks[block.Height]

	if blockData == nil {
		pool.logger.Error("peer sent us a block we didn't expect", "peer", peerID, "curHeight", pool.height, "blockHeight", block.Height)
		return errBadDataFromPeer
	}

	if blockData.peerId != peerID {
		pool.logger.Error("invalid peer", "peer", peerID, "blockHeight", block.Height)
		return errBadDataFromPeer
	}
	if blockData.block != nil {
		pool.logger.Error("already have a block for height", "height", block.Height)
		return errBadDataFromPeer
	}

	pool.blocks[block.Height].block = block
	peer := pool.peers[peerID]
	if peer != nil {
		peer.decrPending(blockSize)
	}
	return nil
}

func (pool *blockPool) getNextTwoBlocks() (first, second *blockData, err error) {

	var block1, block2 *types.Block

	if first = pool.blocks[pool.height]; first != nil {
		block1 = first.block
	}
	if second = pool.blocks[pool.height+1]; second != nil {
		block2 = second.block
	}

	if block1 == nil || block2 == nil {
		// We need both to sync the first block.
		pool.logger.Debug("process blocks doesn't have the blocks", "first", block1, "second", block2)
		err = errMissingBlocks
	}
	return
}

// remove peers that sent us the first two blocks, blocks will also be removed by removePeer()
func (pool *blockPool) invalidateFirstTwoBlocks(err error) {
	if first, ok := pool.blocks[pool.height]; ok {
		pool.removePeer(first.peerId, err)
	}
	if second, ok := pool.blocks[pool.height+1]; ok {
		pool.removePeer(second.peerId, err)
	}
}

func (pool *blockPool) processedCurrentHeightBlock() {
	delete(pool.blocks, pool.height)
	pool.height++
	pool.removeShortPeers()
}

// WIP
// TODO - pace the requests to peers
func (pool *blockPool) sendRequestBatch(sendFunc func(peerID p2p.ID, height int64) error) error {
	if len(pool.blocks) > 30 {
		return nil
	}
	// remove slow and timed out peers
	for _, peer := range pool.peers {
		if err := peer.isGood(); err != nil {
			pool.logger.Info("Removing bad peer", "peer", peer.id, "err", err)
			pool.removePeer(peer.id, err)
			if err == errSlowPeer {
				peer.errFunc(errSlowPeer, peer.id)
			}
		}
	}

	var err error
	// make requests
	for i := 0; i < maxRequestBatchSize; i++ {
		// request height
		height := pool.height + int64(i)
		if height > pool.maxPeerHeight {
			pool.logger.Debug("Will not send request for", "height", height)
			return err
		}
		req := pool.blocks[height]
		if req == nil {
			// make new request
			peerId, err := pool.getBestPeer(height)
			if err != nil {
				// couldn't find a good peer or couldn't communicate with it
				continue
			}
			_ = sendFunc(peerId, height)
		}
	}
	return nil
}

func (pool *blockPool) getBestPeer(height int64) (peerId p2p.ID, err error) {
	// make requests
	// TODO - sort peers in order of goodness
	pool.logger.Debug("try to send request for", "height", height)
	for _, peer := range pool.peers {
		// Send Block Request message to peer
		if peer.height < height {
			continue
		}
		pool.logger.Debug("Try to send request to peer", "peer", peer.id, "height", height)
		if err == errSendQueueFull {
			pool.logger.Error("cannot send request, queue full", "peer", peer.id, "height", height)
			continue
		}
		if err == errNilPeerForBlockRequest {
			// this peer does not exist in the switch, delete locally
			pool.logger.Error("peer doesn't exist in the switch", "peer", peer.id)
			pool.deletePeer(peer.id)
			continue
		}

		pool.logger.Debug("Sent request to peer", "peer", peer.id, "height", height)

		// reserve space for block
		pool.blocks[height] = &blockData{peerId: peer.id, block: nil}
		pool.peers[peer.id].incrPending()
		return peer.id, nil
	}

	return "", errNoPeerFoundForRequest
}
