package p2p

import (
	"errors"
	"sync"
)

// Type of error behaviour a peer can perform.
type ErrorPeerBehaviour int

const (
	ErrorPeerBehaviourUnknown = iota
	ErrorPeerBehaviourBadMessage
	ErrorPeerBehaviourMessageOutofOrder
)

// Type of good behaviour a peer can perform.
type GoodPeerBehaviour int

const (
	GoodPeerBehaviourVote = iota + 100
	GoodPeerBehaviourBlockPart
)

// PeerBehaviour provides an interface for reactors to signal the behaviour
// of peers synchronously to other components.
type PeerBehaviour interface {
	Behaved(peerID ID, reason GoodPeerBehaviour) error
	Errored(peerID ID, reason ErrorPeerBehaviour) error
}

type switchPeerBehaviour struct {
	sw *Switch
}

// Reports the ErrorPeerBehaviour of peer identified by peerID to the Switch.
func (spb *switchPeerBehaviour) Errored(peerID ID, reason ErrorPeerBehaviour) error {
	peer := spb.sw.Peers().Get(peerID)
	if peer == nil {
		return errors.New("Peer not found")
	}

	spb.sw.StopPeerForError(peer, reason)
	return nil
}

// Reports the GoodPeerBehaviour of peer identified by peerID to the Switch.
func (spb *switchPeerBehaviour) Behaved(peerID ID, reason GoodPeerBehaviour) error {
	peer := spb.sw.Peers().Get(peerID)
	if peer == nil {
		return errors.New("Peer not found")
	}

	spb.sw.MarkPeerAsGood(peer)
	return nil
}

// Return a new switchPeerBehaviour instance which wraps the Switch.
func NewSwitchPeerBehaviour(sw *Switch) *switchPeerBehaviour {
	return &switchPeerBehaviour{
		sw: sw,
	}
}

// storedPeerBehaviour serves a mock concrete implementation of the
// PeerBehaviour interface used in reactor tests to ensure reactors
// produce the correct signals in manufactured scenarios.
type storedPeerBehaviour struct {
	eb  map[ID][]ErrorPeerBehaviour
	gb  map[ID][]GoodPeerBehaviour
	mtx sync.RWMutex
}

// StoredPeerBehaviour provides an interface for accessing ErrorPeerBehaviours
// and GoodPeerBehaviour recorded by an implementation of PeerBehaviour
type StoredPeerBehaviour interface {
	GetErrorBehaviours(peerID ID) []ErrorPeerBehaviour
	GetGoodBehaviours(peerID ID) []GoodPeerBehaviour
}

// Creates a new storedPeerBehaviour instance.
func NewStoredPeerBehaviour() *storedPeerBehaviour {
	return &storedPeerBehaviour{
		eb: map[ID][]ErrorPeerBehaviour{},
		gb: map[ID][]GoodPeerBehaviour{},
	}
}

// Stores the ErrorPeerBehaviour produced by the peer identified by peerID.
func (spb *storedPeerBehaviour) Errored(peerID ID, reason ErrorPeerBehaviour) {
	spb.mtx.Lock()
	defer spb.mtx.Unlock()
	if _, ok := spb.eb[peerID]; !ok {
		spb.eb[peerID] = []ErrorPeerBehaviour{reason}
	} else {
		spb.eb[peerID] = append(spb.eb[peerID], reason)
	}
}

// Return all the ErrorBehaviours produced by peer identified by peerID.
func (spb *storedPeerBehaviour) GetErrorBehaviours(peerID ID) []ErrorPeerBehaviour {
	spb.mtx.RLock()
	defer spb.mtx.RUnlock()
	if items, ok := spb.eb[peerID]; ok {
		result := make([]ErrorPeerBehaviour, len(items))
		copy(result, items)

		return result
	} else {
		return []ErrorPeerBehaviour{}
	}
}

// Stores the GoodPeerBehaviour of the peer identified by peerID.
func (spb *storedPeerBehaviour) Behaved(peerID ID, reason GoodPeerBehaviour) {
	spb.mtx.Lock()
	defer spb.mtx.Unlock()
	if _, ok := spb.gb[peerID]; !ok {
		spb.gb[peerID] = []GoodPeerBehaviour{reason}
	} else {
		spb.gb[peerID] = append(spb.gb[peerID], reason)
	}
}

// Returns all the GoodPeerBehaviours produced by the peer identified by peerID.
func (spb *storedPeerBehaviour) GetGoodBehaviours(peerID ID) []GoodPeerBehaviour {
	spb.mtx.RLock()
	defer spb.mtx.RUnlock()
	if items, ok := spb.gb[peerID]; ok {
		result := make([]GoodPeerBehaviour, len(items))
		copy(result, items)

		return result
	} else {
		return []GoodPeerBehaviour{}
	}
}
