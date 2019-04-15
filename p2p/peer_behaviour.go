package p2p

type ErrorPeerBehaviour int

const (
	ErrorPeerBehaviourUnknown = iota
	ErrorPeerBehaviourBadMessage
	ErrorPeerBehaviourMessageOutofOrder
)

type GoodPeerBehaviour int

const (
	GoodPeerBehaviourVote = iota + 100
	GoodPeerBehaviourBlockPart
)

// PeerBehaviour provides an interface for reactors to signal the behaviour
// of peers synchronously to other components.
type PeerBehaviour interface {
	Behaved(peer Peer, reason GoodPeerBehaviour)
	Errored(peer Peer, reason ErrorPeerBehaviour)
}

type switchPeerBehaviour struct {
	sw *Switch
}

func (spb *switchPeerBehaviour) Errored(peer Peer, reason ErrorPeerBehaviour) {
	spb.sw.StopPeerForError(peer, reason)
}

func (spb *switchPeerBehaviour) Behaved(peer Peer, reason GoodPeerBehaviour) {
	spb.sw.MarkPeerAsGood(peer)
}

func NewSwitchPeerBehaviour(sw *Switch) PeerBehaviour {
	return &switchPeerBehaviour{
		sw: sw,
	}
}

type ErrorBehaviours map[Peer][]ErrorPeerBehaviour
type GoodBehaviours map[Peer][]GoodPeerBehaviour

// storedPeerBehaviour serves a mock concrete implementation of the
// PeerBehaviour interface used in reactor tests to ensure reactors
// produce the correct signals in manufactured scenarios.
type storedPeerBehaviour struct {
	eb ErrorBehaviours
	gb GoodBehaviours
}

type StoredPeerBehaviour interface {
    GetErrored() ErrorBehaviours
    GetBehaved() GoodBehaviours
}

func NewStoredPeerBehaviour() *storedPeerBehaviour {
	return &storedPeerBehaviour{
		eb: ErrorBehaviours{},
		gb: GoodBehaviours{},
	}
}

func (spb *storedPeerBehaviour) Errored(peer Peer, reason ErrorPeerBehaviour) {
	if _, ok := spb.eb[peer]; !ok {
		spb.eb[peer] = []ErrorPeerBehaviour{reason}
	} else {
		spb.eb[peer] = append(spb.eb[peer], reason)
	}
}

func (spb *storedPeerBehaviour) GetErrored() ErrorBehaviours {
	return spb.eb
}

func (spb *storedPeerBehaviour) Behaved(peer Peer, reason GoodPeerBehaviour) {
	if _, ok := spb.gb[peer]; !ok {
		spb.gb[peer] = []GoodPeerBehaviour{reason}
	} else {
		spb.gb[peer] = append(spb.gb[peer], reason)
	}
}

func (spb *storedPeerBehaviour) GetBehaved() GoodBehaviours {
	return spb.gb
}
