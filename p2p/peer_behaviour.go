package p2p

import (
	"errors"
	"sync"
)

// PeerBehaviour are types of reportable behaviours about peers.
type PeerBehaviour int

const (
	PeerBehaviourBadMessage = iota
	PeerBehaviourMessageOutOfOrder
	PeerBehaviourVote
	PeerBehaviourBlockPart
)

// PeerBehaviourReporter provides an interface for reactors to report the behaviour
// of peers synchronously to other components.
type PeerBehaviourReporter interface {
	Report(peerID ID, behaviour PeerBehaviour) error
}

// SwitchPeerBehaviouReporter reports peer behaviour to an internal Switch
type SwitchPeerBehaviourReporter struct {
	sw *Switch
}

// Return a new SwitchPeerBehaviourReporter instance which wraps the Switch.
func NewSwitchPeerBehaviourReporter(sw *Switch) *SwitchPeerBehaviourReporter {
	return &SwitchPeerBehaviourReporter{
		sw: sw,
	}
}

// Report reports the behaviour of a peer to the Switch
func (spbr *SwitchPeerBehaviourReporter) Report(peerID ID, behaviour PeerBehaviour) error {
	peer := spbr.sw.Peers().Get(peerID)
	if peer == nil {
		return errors.New("Peer not found")
	}

	switch behaviour {
	case PeerBehaviourVote, PeerBehaviourBlockPart:
		spbr.sw.MarkPeerAsGood(peer)
	case PeerBehaviourBadMessage:
		spbr.sw.StopPeerForError(peer, "Bad message")
	case PeerBehaviourMessageOutOfOrder:
		spbr.sw.StopPeerForError(peer, "Message out of order")
	default:
		return errors.New("Unknown behaviour")
	}

	return nil
}

// MockPeerBehaviourReporter serves a mock concrete implementation of the
// PeerBehaviourReporter interface used in reactor tests to ensure reactors
// report the correct behaviour in manufactured scenarios.
type MockPeerBehaviourReporter struct {
	mtx sync.RWMutex
	pb  map[ID][]PeerBehaviour
}

// NewMockPeerBehaviourReporter returns a PeerBehaviourReporter which records all reported
// behaviours in memory.
func NewMockPeerBehaviourReporter() *MockPeerBehaviourReporter {
	return &MockPeerBehaviourReporter{
		pb: map[ID][]PeerBehaviour{},
	}
}

// Report stores the PeerBehaviour produced by the peer identified by peerID.
func (mpbr *MockPeerBehaviourReporter) Report(peerID ID, behaviour PeerBehaviour) {
	mpbr.mtx.Lock()
	defer mpbr.mtx.Unlock()
	mpbr.pb[peerID] = append(mpbr.pb[peerID], behaviour)
}

// GetBehaviours returns all behaviours reported on the peer identified by peerID.
func (mpbr *MockPeerBehaviourReporter) GetBehaviours(peerID ID) []PeerBehaviour {
	mpbr.mtx.RLock()
	defer mpbr.mtx.RUnlock()
	if items, ok := mpbr.pb[peerID]; ok {
		result := make([]PeerBehaviour, len(items))
		copy(result, items)

		return result
	} else {
		return []PeerBehaviour{}
	}
}
