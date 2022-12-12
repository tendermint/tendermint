# Files and Types

Files and types related to the State/Gossip interaction, excluding testing related.

## reactor.go
- Entry point for the consensus reactor (State+Gossip)
    - `NewReactor`
    - `OnStart`
    - `OnStop`
    - `SwitchToConsensus`

- Implements the Gossip Layer
    - Peer management
        - `InitPeer`
        - `AddPeer`
        - `RemovePeer`

    - Message delivery
        - `Receive`
            - Processes messages delivered by the Gossip layer and updates consensus (`State`) and peer information (`PeerState`)
            - Messages are received through multiple channels
                - `StateChannel`: `NewRoundStepMessage`, `NewValidBlockMessage`, `HasVoteMessage`, `VoteSetMaj23Message`
                - `DataChannel`: `ProposalMessage`, `ProposalPOLMessage`, `BlockPartMessage`
                - `VoteChannel`: `VoteMessage`
                - `VoteBitsChannel`: `VoteSetBitsMessage`

        - `PeerState` update
            - `ApplyNewRoundStepMessage`
            - `ApplyNewValidBlockMessage`
            - `ApplyHasVoteMessage`
            - `SetHasProposal`
            - `ApplyProposalPOLMessage`
            - `SetHasProposalBlockParts`
            - `SetHasVote`
            - `ApplyVoteSetBitsMessage`

        - Peer information requests
            - `VoteSetMaj23Message` (CODE TODO: Move handling from receive to helper function.)

    - [Consensus state](#state.go) update

    - Consensus state monitoring
        - `subscribeToBroadcastEvents` - event bus (Why not simply using channels and go-routines?)
            - `broadcastNewRoundStepMessage`
            - `broadcastNewValidBlockMessage`
            - `broadcastHasVoteMessage`
        - `go updateRoundStateRoutine`


    - [Consensus state](#state.go) gossip 
        - `conR.gossipDataRoutine`
            - `BlockPart`
                 - if peers agree on block parts hash
                 - if peer is lagging, through `gossipDataForCatchup`
            - `Proposal`
            - `ProposalPOL`
	    - `conR.gossipVotesRoutine`
            - `PickSendVote` - sends a `Vote` of the specified set, which depends on the step of self and peer
                - `gossipvotesForHeight` - if H matches
                    - `LastCommit`
                    - POL `Prevotes`
                    - `Prevotes`
                    - `Precommits`

	    - `conR.queryMaj23Routine`

    - Message type definitions
        - ProposalMessage
            - Source: Round proposer
            - Layer: State
            - Step: Propose
            - Once added to the roundState (`PeerState.PeerRoundState.Proposal*`) and accepted (validations passed and all [block parts](`BlockPartMessage`) have been received), will be gossiped.

        - NewRoundStepMessage
        - NewValidBlockMessage

        - ProposalPOLMessage
        - BlockPartMessage
        - VoteMessage
        - HasVoteMessage
        - VoteSetMaj23Message
        - VoteSetBitsMessage
            - Source: full node
            - Step:
            - Sent in response to `VoceSetMaj23Message`; 1-to-1 message.

    - Message forwarding
        - TODO: list

## state.go
- Implements the State layer
    - Keeps the consensus state
        - `State`
    
    - Reacts to messages delivered by the Gossip layer
        - TODO: methods

## msgs.go
Helper functions to convert between wire and go representations of messages.

## types/height_vote_set.go
## types/round_state.go
## types/peer_round_state.go

## metrics.go

## wal.go
Implements the stable storage of messages to allow the recovery of consensus state following crashes.

## ticker.go
Helper to manage the various timeouts used in the consensus algorithm.