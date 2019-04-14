package p2p

type ErrorBehaviourPeer int

const (
	ErrorBehaviourUnknown = iota
	ErrorBehaviourBadMessage
	ErrorBehaviourMessageOutofOrder
)

type GoodBehaviourPeer int

const (
	GoodBehaviourVote = iota
	GoodBehaviourBlockPart
)

// PeerBehaviour provides an interface for reactors to signal the behaviour
// of peers synchronously to other components.
type PeerBehaviour interface {
	Behaved(peer Peer, reason GoodBehaviourPeer)
	Errored(peer Peer, reason ErrorBehaviourPeer)
}

type SwitchedPeerBehaviour struct {
	sw *Switch
}

func (spb *SwitchedPeerBehaviour) Errored(peer Peer, reason ErrorBehaviourPeer) {
	spb.sw.StopPeerForError(peer, reason)
}

func (spb *SwitchedPeerBehaviour) Behaved(peer Peer, reason GoodBehaviourPeer) {
	spb.sw.MarkPeerAsGood(peer)
}

func NewSwitchedPeerBehaviour(sw *Switch) *SwitchedPeerBehaviour {
	return &SwitchedPeerBehaviour{
		sw: sw,
	}
}

type ErrorBehaviours map[Peer][]ErrorBehaviourPeer
type GoodBehaviours map[Peer][]GoodBehaviourPeer

// StorePeerBehaviour serves a mock concrete implementation of the
// PeerBehaviour interface used in reactor tests to ensure reactors
// produce the correct signals in manufactured scenarios.
type StorePeerBehaviour struct {
	eb ErrorBehaviours
	gb GoodBehaviours
}

func NewStorePeerBehaviour() *StorePeerBehaviour {
	return &StorePeerBehaviour{
		eb: make(ErrorBehaviours),
		gb: make(GoodBehaviours),
	}
}

func (spb StorePeerBehaviour) Errored(peer Peer, reason ErrorBehaviourPeer) {
	if _, ok := spb.eb[peer]; !ok {
		spb.eb[peer] = []ErrorBehaviourPeer{reason}
	} else {
		spb.eb[peer] = append(spb.eb[peer], reason)
	}
}

func (mpb *StorePeerBehaviour) GetErrored() ErrorBehaviours {
	return mpb.eb
}

func (spb StorePeerBehaviour) Behaved(peer Peer, reason GoodBehaviourPeer) {
	if _, ok := spb.gb[peer]; !ok {
		spb.gb[peer] = []GoodBehaviourPeer{reason}
	} else {
		spb.gb[peer] = append(spb.gb[peer], reason)
	}
}

func (spb *StorePeerBehaviour) GetBehaved() GoodBehaviours {
	return spb.gb
}
