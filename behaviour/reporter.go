package behaviour

import (
	"errors"

	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
)

// Reporter provides an interface for reactors to report the behaviour
// of peers synchronously to other components.
type Reporter interface {
	Report(behaviour PeerBehaviour) error
}

// SwitchReporter reports peer behaviour to an internal Switch.
type SwitchReporter struct {
	sw *p2p.Switch
}

// NewSwitchReporter return a new SwitchReporter instance which wraps the Switch.
func NewSwitchReporter(sw *p2p.Switch) *SwitchReporter {
	return &SwitchReporter{
		sw: sw,
	}
}

// Report reports the behaviour of a peer to the Switch.
func (spbr *SwitchReporter) Report(behaviour PeerBehaviour) error {
	peer := spbr.sw.Peers().Get(behaviour.peerID)
	if peer == nil {
		return errors.New("peer not found")
	}

	switch reason := behaviour.reason.(type) {
	case consensusVote, blockPart:
		spbr.sw.MarkPeerAsGood(peer)
	case badMessage:
		spbr.sw.StopPeerForError(peer, reason.explanation)
	case messageOutOfOrder:
		spbr.sw.StopPeerForError(peer, reason.explanation)
	default:
		return errors.New("unknown reason reported")
	}

	return nil
}

// MockReporter is a concrete implementation of the Reporter
// interface used in reactor tests to ensure reactors report the correct
// behaviour in manufactured scenarios.
type MockReporter struct {
	mtx tmsync.RWMutex
	pb  map[p2p.ID][]PeerBehaviour
}

// NewMockReporter returns a Reporter which records all reported
// behaviours in memory.
func NewMockReporter() *MockReporter {
	return &MockReporter{
		pb: map[p2p.ID][]PeerBehaviour{},
	}
}

// Report stores the PeerBehaviour produced by the peer identified by peerID.
func (mpbr *MockReporter) Report(behaviour PeerBehaviour) error {
	mpbr.mtx.Lock()
	defer mpbr.mtx.Unlock()
	mpbr.pb[behaviour.peerID] = append(mpbr.pb[behaviour.peerID], behaviour)

	return nil
}

// GetBehaviours returns all behaviours reported on the peer identified by peerID.
func (mpbr *MockReporter) GetBehaviours(peerID p2p.ID) []PeerBehaviour {
	mpbr.mtx.RLock()
	defer mpbr.mtx.RUnlock()
	if items, ok := mpbr.pb[peerID]; ok {
		result := make([]PeerBehaviour, len(items))
		copy(result, items)

		return result
	}

	return []PeerBehaviour{}
}
