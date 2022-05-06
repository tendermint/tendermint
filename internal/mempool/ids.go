package mempool

import (
	"fmt"
	"sync"

	"github.com/tendermint/tendermint/types"
)

type IDs struct {
	mtx       sync.RWMutex
	peerMap   map[types.NodeID]uint16
	nextID    uint16              // assumes that a node will never have over 65536 active peers
	activeIDs map[uint16]struct{} // used to check if a given peerID key is used
}

func NewMempoolIDs() *IDs {
	return &IDs{
		peerMap: make(map[types.NodeID]uint16),

		// reserve UnknownPeerID for mempoolReactor.BroadcastTx
		activeIDs: map[uint16]struct{}{UnknownPeerID: {}},
		nextID:    1,
	}
}

// ReserveForPeer searches for the next unused ID and assigns it to the provided
// peer.
func (ids *IDs) ReserveForPeer(peerID types.NodeID) {
	ids.mtx.Lock()
	defer ids.mtx.Unlock()

	if _, ok := ids.peerMap[peerID]; ok {
		// the peer has been reserved
		return
	}

	curID := ids.nextPeerID()
	ids.peerMap[peerID] = curID
	ids.activeIDs[curID] = struct{}{}
}

// Reclaim returns the ID reserved for the peer back to unused pool.
func (ids *IDs) Reclaim(peerID types.NodeID) {
	ids.mtx.Lock()
	defer ids.mtx.Unlock()

	removedID, ok := ids.peerMap[peerID]
	if ok {
		delete(ids.activeIDs, removedID)
		delete(ids.peerMap, peerID)
		if removedID < ids.nextID {
			ids.nextID = removedID
		}
	}
}

// GetForPeer returns an ID reserved for the peer.
func (ids *IDs) GetForPeer(peerID types.NodeID) uint16 {
	ids.mtx.RLock()
	defer ids.mtx.RUnlock()

	return ids.peerMap[peerID]
}

// nextPeerID returns the next unused peer ID to use. We assume that the mutex
// is already held.
func (ids *IDs) nextPeerID() uint16 {
	if len(ids.activeIDs) == MaxActiveIDs {
		panic(fmt.Sprintf("node has maximum %d active IDs and wanted to get one more", MaxActiveIDs))
	}

	_, idExists := ids.activeIDs[ids.nextID]
	for idExists {
		ids.nextID++
		_, idExists = ids.activeIDs[ids.nextID]
	}

	curID := ids.nextID
	ids.nextID++

	return curID
}
