# Consensus Reactor

Consensus Reactor defines a reactor for the consensus service. It contains the ConsensusState service that
manages the state of the Tendermint consensus internal state machine.
When Consensus Reactor is started, it starts Broadcast Routine which starts ConsensusState service.
Furthermore, for each peer that is added to the Consensus Reactor, it creates (and manages) the known peer state
(that is used extensively in gossip routines) and starts the following three routines for the peer p:
Gossip Data Routine, Gossip Votes Routine and QueryMaj23Routine. Finally, Consensus Reactor is responsible
for decoding messages received from a peer and for adequate processing of the message depending on its type and content.
The processing normally consists of updating the known peer state and for some messages
(`ProposalMessage`, `BlockPartMessage` and `VoteMessage`) also forwarding message to ConsensusState module
for further processing. In the following text we specify the core functionality of those separate unit of executions
that are part of the Consensus Reactor.

## ConsensusState service

Consensus State handles execution of the Tendermint BFT consensus algorithm. It processes votes and proposals,
and upon reaching agreement, commits blocks to the chain and executes them against the application.
The internal state machine receives input from peers, the internal validator and from a timer.

Inside Consensus State we have the following units of execution: Timeout Ticker and Receive Routine.
Timeout Ticker is a timer that schedules timeouts conditional on the height/round/step that are processed
by the Receive Routine.

### Receive Routine of the ConsensusState service

Receive Routine of the ConsensusState handles messages which may cause internal consensus state transitions.
It is the only routine that updates RoundState that contains internal consensus state.
Updates (state transitions) happen on timeouts, complete proposals, and 2/3 majorities.
It receives messages from peers, internal validators and from Timeout Ticker
and invokes the corresponding handlers, potentially updating the RoundState.
The details of the protocol (together with formal proofs of correctness) implemented by the Receive Routine are
discussed in separate document. For understanding of this document
it is sufficient to understand that the Receive Routine manages and updates RoundState data structure that is
then extensively used by the gossip routines to determine what information should be sent to peer processes.

## Round State

RoundState defines the internal consensus state. It contains height, round, round step, a current validator set,
a proposal and proposal block for the current round, locked round and block (if some block is being locked), set of
received votes and last commit and last validators set.

```golang
type RoundState struct {
	Height             int64
	Round              int
	Step               RoundStepType
	Validators         ValidatorSet
	Proposal           Proposal
	ProposalBlock      Block
	ProposalBlockParts PartSet
	LockedRound        int
	LockedBlock        Block
	LockedBlockParts   PartSet
	Votes              HeightVoteSet
	LastCommit         VoteSet
	LastValidators     ValidatorSet
}
```

Internally, consensus will run as a state machine with the following states:

- RoundStepNewHeight
- RoundStepNewRound
- RoundStepPropose
- RoundStepProposeWait
- RoundStepPrevote
- RoundStepPrevoteWait
- RoundStepPrecommit
- RoundStepPrecommitWait
- RoundStepCommit

## Peer Round State

Peer round state contains the known state of a peer. It is being updated by the Receive routine of
Consensus Reactor and by the gossip routines upon sending a message to the peer.

```golang
type PeerRoundState struct {
	Height                   int64               // Height peer is at
	Round                    int                 // Round peer is at, -1 if unknown.
	Step                     RoundStepType       // Step peer is at
	Proposal                 bool                // True if peer has proposal for this round
	ProposalBlockPartsHeader PartSetHeader
	ProposalBlockParts       BitArray
	ProposalPOLRound         int                 // Proposal's POL round. -1 if none.
	ProposalPOL              BitArray            // nil until ProposalPOLMessage received.
	Prevotes                 BitArray            // All votes peer has for this round
	Precommits               BitArray            // All precommits peer has for this round
	LastCommitRound          int                 // Round of commit for last height. -1 if none.
	LastCommit               BitArray            // All commit precommits of commit for last height.
	CatchupCommitRound       int                 // Round that we have commit for. Not necessarily unique. -1 if none.
	CatchupCommit            BitArray            // All commit precommits peer has for this height & CatchupCommitRound
}
```

## Receive method of Consensus reactor

The entry point of the Consensus reactor is a receive method. When a message is received from a peer p,
normally the peer round state is updated correspondingly, and some messages
are passed for further processing, for example to ConsensusState service. We now specify the processing of messages
in the receive method of Consensus reactor for each message type. In the following message handler, `rs` and `prs` denote
`RoundState` and `PeerRoundState`, respectively.

### NewRoundStepMessage handler

```
handleMessage(msg):
    if msg is from smaller height/round/step then return
    // Just remember these values.
    prsHeight = prs.Height
    prsRound = prs.Round
    prsCatchupCommitRound = prs.CatchupCommitRound
    prsCatchupCommit = prs.CatchupCommit

    Update prs with values from msg
    if prs.Height or prs.Round has been updated then
        reset Proposal related fields of the peer state
    if prs.Round has been updated and msg.Round == prsCatchupCommitRound then
        prs.Precommits = psCatchupCommit
    if prs.Height has been updated then
        if prsHeight+1 == msg.Height && prsRound == msg.LastCommitRound then
            prs.LastCommitRound = msg.LastCommitRound
        	prs.LastCommit = prs.Precommits
        } else {
            prs.LastCommitRound = msg.LastCommitRound
        	prs.LastCommit = nil
        }
        Reset prs.CatchupCommitRound and prs.CatchupCommit
```

### NewValidBlockMessage handler

```
handleMessage(msg):
    if prs.Height != msg.Height then return
    
    if prs.Round != msg.Round && !msg.IsCommit then return
    
    prs.ProposalBlockPartsHeader = msg.BlockPartsHeader
    prs.ProposalBlockParts = msg.BlockParts
```

### HasVoteMessage handler

```
handleMessage(msg):
    if prs.Height == msg.Height then
        prs.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
```

### VoteSetMaj23Message handler

```
handleMessage(msg):
    if prs.Height == msg.Height then
        Record in rs that a peer claim to have ⅔ majority for msg.BlockID
        Send VoteSetBitsMessage showing votes node has for that BlockId
```

### ProposalMessage handler

```
handleMessage(msg):
    if prs.Height != msg.Height || prs.Round != msg.Round || prs.Proposal then return
    prs.Proposal = true
    if prs.ProposalBlockParts == empty set then // otherwise it is set in NewValidBlockMessage handler
      prs.ProposalBlockPartsHeader = msg.BlockPartsHeader
    prs.ProposalPOLRound = msg.POLRound
    prs.ProposalPOL = nil
    Send msg through internal peerMsgQueue to ConsensusState service
```

### ProposalPOLMessage handler

```
handleMessage(msg):
    if prs.Height != msg.Height or prs.ProposalPOLRound != msg.ProposalPOLRound then return
    prs.ProposalPOL = msg.ProposalPOL
```

### BlockPartMessage handler

```
handleMessage(msg):
    if prs.Height != msg.Height || prs.Round != msg.Round then return
    Record in prs that peer has block part msg.Part.Index
    Send msg trough internal peerMsgQueue to ConsensusState service
```

### VoteMessage handler

```
handleMessage(msg):
    Record in prs that a peer knows vote with index msg.vote.ValidatorIndex for particular height and round
    Send msg trough internal peerMsgQueue to ConsensusState service
```

### VoteSetBitsMessage handler

```
handleMessage(msg):
    Update prs for the bit-array of votes peer claims to have for the msg.BlockID
```

## Gossip Data Routine

It is used to send the following messages to the peer: `BlockPartMessage`, `ProposalMessage` and
`ProposalPOLMessage` on the DataChannel. The gossip data routine is based on the local RoundState (`rs`)
and the known PeerRoundState (`prs`). The routine repeats forever the logic shown below:

```
1a) if rs.ProposalBlockPartsHeader == prs.ProposalBlockPartsHeader and the peer does not have all the proposal parts then
        Part = pick a random proposal block part the peer does not have
        Send BlockPartMessage(rs.Height, rs.Round, Part) to the peer on the DataChannel
        if send returns true, record that the peer knows the corresponding block Part
	    Continue

1b) if (0 < prs.Height) and (prs.Height < rs.Height) then
        help peer catch up using gossipDataForCatchup function
        Continue

1c) if (rs.Height != prs.Height) or (rs.Round != prs.Round) then
        Sleep PeerGossipSleepDuration
        Continue

//  at this point rs.Height == prs.Height and rs.Round == prs.Round
1d) if (rs.Proposal != nil and !prs.Proposal) then
        Send ProposalMessage(rs.Proposal) to the peer
        if send returns true, record that the peer knows Proposal
	    if 0 <= rs.Proposal.POLRound then
	    polRound = rs.Proposal.POLRound
        prevotesBitArray = rs.Votes.Prevotes(polRound).BitArray()
        Send ProposalPOLMessage(rs.Height, polRound, prevotesBitArray)
        Continue

2)  Sleep PeerGossipSleepDuration
```

### Gossip Data For Catchup

This function is responsible for helping peer catch up if it is at the smaller height (prs.Height < rs.Height).
The function executes the following logic:

    if peer does not have all block parts for prs.ProposalBlockPart then
        blockMeta =  Load Block Metadata for height prs.Height from blockStore
        if (!blockMeta.BlockID.PartsHeader == prs.ProposalBlockPartsHeader) then
            Sleep PeerGossipSleepDuration
    	return
        Part = pick a random proposal block part the peer does not have
        Send BlockPartMessage(prs.Height, prs.Round, Part) to the peer on the DataChannel
        if send returns true, record that the peer knows the corresponding block Part
        return
    else Sleep PeerGossipSleepDuration

## Gossip Votes Routine

It is used to send the following message: `VoteMessage` on the VoteChannel.
The gossip votes routine is based on the local RoundState (`rs`)
and the known PeerRoundState (`prs`). The routine repeats forever the logic shown below:

```
1a) if rs.Height == prs.Height then
        if prs.Step == RoundStepNewHeight then
            vote = random vote from rs.LastCommit the peer does not have
            Send VoteMessage(vote) to the peer
            if send returns true, continue

        if prs.Step <= RoundStepPrevote and prs.Round != -1 and prs.Round <= rs.Round then
            Prevotes = rs.Votes.Prevotes(prs.Round)
            vote = random vote from Prevotes the peer does not have
            Send VoteMessage(vote) to the peer
            if send returns true, continue

        if prs.Step <= RoundStepPrecommit and prs.Round != -1 and prs.Round <= rs.Round then
     	    Precommits = rs.Votes.Precommits(prs.Round)
            vote = random vote from Precommits the peer does not have
            Send VoteMessage(vote) to the peer
            if send returns true, continue

        if prs.ProposalPOLRound != -1 then
            PolPrevotes = rs.Votes.Prevotes(prs.ProposalPOLRound)
            vote = random vote from PolPrevotes the peer does not have
            Send VoteMessage(vote) to the peer
            if send returns true, continue

1b)  if prs.Height != 0 and rs.Height == prs.Height+1 then
        vote = random vote from rs.LastCommit peer does not have
        Send VoteMessage(vote) to the peer
        if send returns true, continue

1c)  if prs.Height != 0 and rs.Height >= prs.Height+2 then
        Commit = get commit from BlockStore for prs.Height
        vote = random vote from Commit the peer does not have
        Send VoteMessage(vote) to the peer
        if send returns true, continue

2)   Sleep PeerGossipSleepDuration
```

## QueryMaj23Routine

It is used to send the following message: `VoteSetMaj23Message`. `VoteSetMaj23Message` is sent to indicate that a given
BlockID has seen +2/3 votes. This routine is based on the local RoundState (`rs`) and the known PeerRoundState
(`prs`). The routine repeats forever the logic shown below.

```
1a) if rs.Height == prs.Height then
        Prevotes = rs.Votes.Prevotes(prs.Round)
        if there is a ⅔ majority for some blockId in Prevotes then
        m = VoteSetMaj23Message(prs.Height, prs.Round, Prevote, blockId)
        Send m to peer
        Sleep PeerQueryMaj23SleepDuration

1b) if rs.Height == prs.Height then
        Precommits = rs.Votes.Precommits(prs.Round)
        if there is a ⅔ majority for some blockId in Precommits then
        m = VoteSetMaj23Message(prs.Height,prs.Round,Precommit,blockId)
        Send m to peer
        Sleep PeerQueryMaj23SleepDuration

1c) if rs.Height == prs.Height and prs.ProposalPOLRound >= 0 then
        Prevotes = rs.Votes.Prevotes(prs.ProposalPOLRound)
        if there is a ⅔ majority for some blockId in Prevotes then
        m = VoteSetMaj23Message(prs.Height,prs.ProposalPOLRound,Prevotes,blockId)
        Send m to peer
        Sleep PeerQueryMaj23SleepDuration

1d) if prs.CatchupCommitRound != -1 and 0 < prs.Height and
        prs.Height <= blockStore.Height() then
        Commit = LoadCommit(prs.Height)
        m = VoteSetMaj23Message(prs.Height,Commit.Round,Precommit,Commit.blockId)
        Send m to peer
        Sleep PeerQueryMaj23SleepDuration

2)  Sleep PeerQueryMaj23SleepDuration
```

## Broadcast routine

The Broadcast routine subscribes to an internal event bus to receive new round steps and votes messages, and broadcasts messages to peers upon receiving those 
events.
It broadcasts `NewRoundStepMessage` or `CommitStepMessage` upon new round state event. Note that
broadcasting these messages does not depend on the PeerRoundState; it is sent on the StateChannel.
Upon receiving VoteMessage it broadcasts `HasVoteMessage` message to its peers on the StateChannel.

## Channels

Defines 4 channels: state, data, vote and vote_set_bits. Each channel
has `SendQueueCapacity` and `RecvBufferCapacity` and
`RecvMessageCapacity` set to `maxMsgSize`.

Sending incorrectly encoded data will result in stopping the peer.
