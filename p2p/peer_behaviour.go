package p2p

type ErrPeer int

const (
	ErrPeerUnknown = iota
	ErrPeerBadMessage
	ErrPeerMessageOutofOrder
)

type PeerBehaviour interface {
	Errored(peer Peer, reason ErrPeer)
	MarkPeerAsGood(peer Peer)
}

type SwitchedPeerBehaviour struct {
	sw *Switch
}

func (spb *SwitchedPeerBehaviour) Errored(peer Peer, reason ErrPeer) {
	spb.sw.StopPeerForError(peer, reason)
}

func (spb *SwitchedPeerBehaviour) MarkPeerAsGood(peer Peer) {
	spb.sw.MarkPeerAsGood(peer)
}

func NewSwitchedPeerBehaviour(sw *Switch) *SwitchedPeerBehaviour {
	return &SwitchedPeerBehaviour{
		sw: sw,
	}
}

type PeerErrors map[Peer][]ErrPeer
type GoodPeers map[Peer]bool

type StorePeerBehaviour struct {
	pe PeerErrors
	gp GoodPeers
}

func NewStorePeerBehaviour() *StorePeerBehaviour {
	return &StorePeerBehaviour{
		pe: make(PeerErrors),
		gp: GoodPeers{},
	}
}

func (spb StorePeerBehaviour) Errored(peer Peer, reason ErrPeer) {
	if _, ok := spb.pe[peer]; !ok {
		spb.pe[peer] = []ErrPeer{reason}
	} else {
		spb.pe[peer] = append(spb.pe[peer], reason)
	}
}

func (mpb *StorePeerBehaviour) GetPeerErrors() PeerErrors {
	return mpb.pe
}

func (spb *StorePeerBehaviour) MarkPeerAsGood(peer Peer) {
	if _, ok := spb.gp[peer]; !ok {
		spb.gp[peer] = true
	}
}

func (spb *StorePeerBehaviour) GetGoodPeers() GoodPeers {
	return spb.gp
}
