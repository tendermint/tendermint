package v2

import "github.com/tendermint/tendermint/p2p"

/*
Mocking out IO

	* Should we include an IO routine?
	* Advantage: We Can Mock it out for integration tests

	What would the IO routine do?
		* sendStatusRequest
		* sendBlockRequest
		* reportPeerError
*/

/*
Dependencies:
* switch:
	Reactor("CONSENSUS")
	Peers().Get(peerID)
	sendBlockRequest
		Switch.Peers()(peerID)

*/
type io struct {
}

func (i *io) sendBlockRequest(peerID p2p.ID, height int64) error {
	peer := bcR.Switch.Peers().Get(peerID)
	if peer == nil {
		return errNilPeerForBlockRequest
	}

	msgBytes := cdc.MustMarshalBinaryBare(&bcBlockRequestMessage{height})
	queued := peer.TrySend(BlockchainChannel, msgBytes)
	if !queued {
		return errSendQueueFull
	}
	return nil
}

func (i *io) sendStatusRequest() {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{bcR.store.Height()})
	i.Switch.Broadcast(BlockchainChannel, msgBytes)
}

func (bcR *BlockchainReactor) switchToConsensus() {
	conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
	if ok {
		conR.SwitchToConsensus(bcR.state, bcR.blocksSynced)
		bcR.eventsFromFSMCh <- bcFsmMessage{event: syncFinishedEv}
	}
	// else {
	// Should only happen during testing.
	// }
}

// XXX: Broadcast status?
// XXX: should take a height instead of
func (bcR *BlockchainReactor) sendStatusRequest() {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{bcR.store.Height()})
	bcR.Switch.Broadcast(BlockchainChannel, msgBytes)
}
