package behavior

import (
	"errors"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

// Reporter provides an interface for reactors to report the behavior
// of peers synchronously to other components.
type Reporter interface {
	Report(behavior PeerBehavior) error
}

// SwitchReporter reports peer behavior to an internal Switch.
type SwitchReporter struct {
	sw *p2p.Switch
}

// NewSwitchReporter return a new SwitchReporter instance which wraps the Switch.
func NewSwitchReporter(sw *p2p.Switch) *SwitchReporter {
	return &SwitchReporter{
		sw: sw,
	}
}

// Report reports the behavior of a peer to the Switch.
func (spbr *SwitchReporter) Report(behavior PeerBehavior) error {
	peer := spbr.sw.Peers().Get(behavior.peerID)
	if peer == nil {
		return errors.New("peer not found")
	}

	switch reason := behavior.reason.(type) {
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
// behavior in manufactured scenarios.
type MockReporter struct {
	mtx tmsync.RWMutex
	pb  map[types.NodeID][]PeerBehavior
}

// NewMockReporter returns a Reporter which records all reported
// behaviors in memory.
func NewMockReporter() *MockReporter {
	return &MockReporter{
		pb: map[types.NodeID][]PeerBehavior{},
	}
}

// Report stores the PeerBehavior produced by the peer identified by peerID.
func (mpbr *MockReporter) Report(behavior PeerBehavior) error {
	mpbr.mtx.Lock()
	defer mpbr.mtx.Unlock()
	mpbr.pb[behavior.peerID] = append(mpbr.pb[behavior.peerID], behavior)

	return nil
}

// GetBehaviors returns all behaviors reported on the peer identified by peerID.
func (mpbr *MockReporter) GetBehaviors(peerID types.NodeID) []PeerBehavior {
	mpbr.mtx.RLock()
	defer mpbr.mtx.RUnlock()
	if items, ok := mpbr.pb[peerID]; ok {
		result := make([]PeerBehavior, len(items))
		copy(result, items)

		return result
	}

	return []PeerBehavior{}
}
