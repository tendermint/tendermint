package behaviour

import (
	"github.com/tendermint/tendermint/p2p"
)

// PeerBehaviour is a struct describing the behaviour of a peer performed.
type PeerBehaviour struct {
	peerID p2p.ID
	reason interface{}
}

type badMessage struct {
	explanation string
}

// BadMessage returns a badMessage PeerBehaviour.
func BadMessage(peerID p2p.ID, explanation string) PeerBehaviour {
	return PeerBehaviour{peerID: peerID, reason: badMessage{explanation}}
}

type messageOutOfOrder struct {
	explanation string
}

// MessageOutOfOrder returns a messagOutOfOrder PeerBehaviour.
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

// BlockPart returns blockPart PeerBehaviour.
func BlockPart(peerID p2p.ID, explanation string) PeerBehaviour {
	return PeerBehaviour{peerID: peerID, reason: blockPart{explanation}}
}
