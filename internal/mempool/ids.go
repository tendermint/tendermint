package mempool

import (
	"fmt"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/types"
)

// nolint: revive
// TODO: Rename type.
type MempoolIDs struct {
	mtx       tmsync.RWMutex
	peerMap   map[types.NodeID]uint16
	nextID    uint16              // assumes that a node will never have over 65536 active peers
	activeIDs map[uint16]struct{} // used to check if a given peerID key is used
}

func NewMempoolIDs() *MempoolIDs {
	return &MempoolIDs{
		peerMap: make(map[types.NodeID]uint16),

		// reserve UnknownPeerID for mempoolReactor.BroadcastTx
		activeIDs: map[uint16]struct{}{UnknownPeerID: {}},
		nextID:    1,
	}
}

// ReserveForPeer searches for the next unused ID and assigns it to the provided
// peer.
func (ids *MempoolIDs) ReserveForPeer(peerID types.NodeID) {
	ids.mtx.Lock()
	defer ids.mtx.Unlock()

	curID := ids.nextPeerID()
	ids.peerMap[peerID] = curID
	ids.activeIDs[curID] = struct{}{}
}

// Reclaim returns the ID reserved for the peer back to unused pool.
func (ids *MempoolIDs) Reclaim(peerID types.NodeID) {
	ids.mtx.Lock()
	defer ids.mtx.Unlock()

	removedID, ok := ids.peerMap[peerID]
	if ok {
		delete(ids.activeIDs, removedID)
		delete(ids.peerMap, peerID)
	}
}

// GetForPeer returns an ID reserved for the peer.
func (ids *MempoolIDs) GetForPeer(peerID types.NodeID) uint16 {
	ids.mtx.RLock()
	defer ids.mtx.RUnlock()

	return ids.peerMap[peerID]
}

// nextPeerID returns the next unused peer ID to use. We assume that the mutex
// is already held.
func (ids *MempoolIDs) nextPeerID() uint16 {
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
