package behavior

import (
	"github.com/tendermint/tendermint/p2p"
)

// PeerBehavior is a struct describing a behavior a peer performed.
// `peerID` identifies the peer and reason characterizes the specific
// behavior performed by the peer.
type PeerBehavior struct {
	peerID p2p.ID
	reason interface{}
}

type badMessage struct {
	explanation string
}

// BadMessage returns a badMessage PeerBehavior.
func BadMessage(peerID p2p.ID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: badMessage{explanation}}
}

type messageOutOfOrder struct {
	explanation string
}

// MessageOutOfOrder returns a messagOutOfOrder PeerBehavior.
func MessageOutOfOrder(peerID p2p.ID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: messageOutOfOrder{explanation}}
}

type consensusVote struct {
	explanation string
}

// ConsensusVote returns a consensusVote PeerBehavior.
func ConsensusVote(peerID p2p.ID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: consensusVote{explanation}}
}

type blockPart struct {
	explanation string
}

// BlockPart returns blockPart PeerBehavior.
func BlockPart(peerID p2p.ID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: blockPart{explanation}}
}
