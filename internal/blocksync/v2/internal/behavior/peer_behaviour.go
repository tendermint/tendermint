package behavior

import "github.com/tendermint/tendermint/types"

// PeerBehavior is a struct describing a behavior a peer performed.
// `peerID` identifies the peer and reason characterizes the specific
// behavior performed by the peer.
type PeerBehavior struct {
	peerID types.NodeID
	reason interface{}
}

type badMessage struct {
	explanation string
}

// BadMessage returns a badMessage PeerBehavior.
func BadMessage(peerID types.NodeID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: badMessage{explanation}}
}

type messageOutOfOrder struct {
	explanation string
}

// MessageOutOfOrder returns a messagOutOfOrder PeerBehavior.
func MessageOutOfOrder(peerID types.NodeID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: messageOutOfOrder{explanation}}
}

type consensusVote struct {
	explanation string
}

// ConsensusVote returns a consensusVote PeerBehavior.
func ConsensusVote(peerID types.NodeID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: consensusVote{explanation}}
}

type blockPart struct {
	explanation string
}

// BlockPart returns blockPart PeerBehavior.
func BlockPart(peerID types.NodeID, explanation string) PeerBehavior {
	return PeerBehavior{peerID: peerID, reason: blockPart{explanation}}
}
