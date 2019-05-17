package behaviour

import (
	"errors"
	"sync"

	"github.com/tendermint/tendermint/p2p"
)

type PeerBehaviour struct {
	peerID p2p.ID
	reason interface{}
}

type badMessage struct {
	explanation string
}

func BadMessage(peerID p2p.ID, explanation string) PeerBehaviour {
	return PeerBehaviour{peerID: peerID, reason: badMessage{explanation}}
}

type messageOutOfOrder struct {
	explanation string
}

func MessageOutOfOrder(peerID p2p.ID, explanation string) PeerBehaviour {
	return PeerBehaviour{peerID: peerID, reason: badMessage{explanation}}
}

type consensusVote struct {
	explanation string
}

// ConsensusVote creates a PeerBehaviour with a consensusVote reason.
func ConsensusVote(peerID p2p.ID, explanation string) PeerBehaviour {
	return PeerBehaviour{peerID: peerID, reason: consensusVote{explanation}}
}

type blockPart struct {
	explanation string
}

// BlockPart creates a PeerBehaviour with a blockPart reason.
func BlockPart(peerID p2p.ID, explanation string) PeerBehaviour {
	return PeerBehaviour{peerID: peerID, reason: blockPart{explanation}}
}

// PeerBehaviourReporter provides an interface for reactors to report the behaviour
// of peers synchronously to other components.
type PeerBehaviourReporter interface {
	Report(behaviour PeerBehaviour) error
}

// SwitchPeerBehaviouReporter reports peer behaviour to an internal Switch
type SwitchPeerBehaviourReporter struct {
	sw *p2p.Switch
}

// Return a new SwitchPeerBehaviourReporter instance which wraps the Switch.
func NewSwitchPeerBehaviourReporter(sw *p2p.Switch) *SwitchPeerBehaviourReporter {
	return &SwitchPeerBehaviourReporter{
		sw: sw,
	}
}

// Report reports the behaviour of a peer to the Switch
func (spbr *SwitchPeerBehaviourReporter) Report(behaviour PeerBehaviour) error {
	peer := spbr.sw.Peers().Get(behaviour.peerID)
	if peer == nil {
		return errors.New("Peer not found")
	}

	switch behaviour.reason.(type) {
	case consensusVote, blockPart:
		spbr.sw.MarkPeerAsGood(peer)
	case badMessage:
		spbr.sw.StopPeerForError(peer, "Bad message")
	case messageOutOfOrder:
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
	pb  map[p2p.ID][]PeerBehaviour
}

// NewMockPeerBehaviourReporter returns a PeerBehaviourReporter which records all reported
// behaviours in memory.
func NewMockPeerBehaviourReporter() *MockPeerBehaviourReporter {
	return &MockPeerBehaviourReporter{
		pb: map[p2p.ID][]PeerBehaviour{},
	}
}

// Report stores the PeerBehaviour produced by the peer identified by peerID.
func (mpbr *MockPeerBehaviourReporter) Report(behaviour PeerBehaviour) {
	mpbr.mtx.Lock()
	defer mpbr.mtx.Unlock()
	mpbr.pb[behaviour.peerID] = append(mpbr.pb[behaviour.peerID], behaviour)
}

// GetBehaviours returns all behaviours reported on the peer identified by peerID.
func (mpbr *MockPeerBehaviourReporter) GetBehaviours(peerID p2p.ID) []PeerBehaviour {
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
