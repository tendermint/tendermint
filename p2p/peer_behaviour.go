package p2p

import (
	"errors"
	"sync"
)

// PeerBehaviour are types of reportable behaviours by peers.
type PeerBehaviour int

const (
	PeerBehaviourBadMessage = iota
	PeerBehaviourMessageOutOfOrder
	PeerBehaviourVote
	PeerBehaviourBlockPart
)

// PeerReporter provides an interface for reactors to report the behaviour
// of peers synchronously to other components.
type PeerReporter interface {
	Report(peerID ID, behaviour PeerBehaviour) error
}

// SwitchPeerReporter reports peer behaviour to an internal Switch
type SwitchPeerReporter struct {
	sw *Switch
}

// Return a new switchPeerBehaviour instance which wraps the Switch.
func NewSwitchPeerReporter(sw *Switch) *SwitchPeerReporter {
	return &SwitchPeerReporter{
		sw: sw,
	}
}

// Report reports the behaviour of a peer to the Switch
func (spb *SwitchPeerReporter) Report(peerID ID, behaviour PeerBehaviour) error {
	peer := spb.sw.Peers().Get(peerID)
	if peer == nil {
		return errors.New("Peer not found")
	}

	switch behaviour {
	case PeerBehaviourVote, PeerBehaviourBlockPart:
		spb.sw.MarkPeerAsGood(peer)
	case PeerBehaviourBadMessage:
		spb.sw.StopPeerForError(peer, "Bad message")
	case PeerBehaviourMessageOutOfOrder:
		spb.sw.StopPeerForError(peer, "Message out of order")
	default:
		return errors.New("Unknown behaviour")
	}

	return nil
}

// MockPeerBehaviour serves a mock concrete implementation of the
// PeerReporter interface used in reactor tests to ensure reactors
// report the correct behaviour in manufactured scenarios.
type MockPeerReporter struct {
	mtx sync.RWMutex
	pb  map[ID][]PeerBehaviour
}

// NewMockPeerReporter returns a PeerReporter which records all reported
// behaviours in memory.
func NewMockPeerReporter() *MockPeerReporter {
	return &MockPeerReporter{
		pb: map[ID][]PeerBehaviour{},
	}
}

// Report stores the PeerBehaviour produced by the peer identified by ID.
func (mpr *MockPeerReporter) Report(peerID ID, behaviour PeerBehaviour) {
	mpr.mtx.Lock()
	defer mpr.mtx.Unlock()
	mpr.pb[peerID] = append(mpr.pb[peerID], behaviour)
}

// GetBehaviours returns all behaviours reported on the peer identified by peerID.
func (mpr *MockPeerReporter) GetBehaviours(peerID ID) []PeerBehaviour {
	mpr.mtx.RLock()
	defer mpr.mtx.RUnlock()
	if items, ok := mpr.pb[peerID]; ok {
		result := make([]PeerBehaviour, len(items))
		copy(result, items)

		return result
	} else {
		return []PeerBehaviour{}
	}
}
