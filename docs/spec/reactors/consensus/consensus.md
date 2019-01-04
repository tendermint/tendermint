# Tendermint Consensus Reactor

Tendermint Consensus is a distributed protocol executed by validator processes to agree on
the next block to be added to the Tendermint blockchain. The protocol proceeds in rounds, where
each round is a try to reach agreement on the next block. A round starts by having a dedicated
process (called proposer) suggesting to other processes what should be the next block with
the `ProposalMessage`.
The processes respond by voting for a block with `VoteMessage` (there are two kinds of vote
messages, prevote and precommit votes). Note that a proposal message is just a suggestion what the
next block should be; a validator might vote with a `VoteMessage` for a different block. If in some
round, enough number of processes vote for the same block, then this block is committed and later
added to the blockchain. `ProposalMessage` and `VoteMessage` are signed by the private key of the
validator. The internals of the protocol and how it ensures safety and liveness properties are
explained in a forthcoming document.

For efficiency reasons, validators in Tendermint consensus protocol do not agree directly on the
block as the block size is big, i.e., they don't embed the block inside `Proposal` and
`VoteMessage`. Instead, they reach agreement on the `BlockID` (see `BlockID` definition in
[Blockchain](https://github.com/tendermint/tendermint/blob/master/docs/spec/blockchain/blockchain.md#blockid) section) that uniquely identifies each block. The block itself is
disseminated to validator processes using peer-to-peer gossiping protocol. It starts by having a
proposer first splitting a block into a number of block parts, that are then gossiped between
processes using `BlockPartMessage`.

Validators in Tendermint communicate by peer-to-peer gossiping protocol. Each validator is connected
only to a subset of processes called peers. By the gossiping protocol, a validator send to its peers
all needed information (`ProposalMessage`, `VoteMessage` and `BlockPartMessage`) so they can
reach agreement on some block, and also obtain the content of the chosen block (block parts). As
part of the gossiping protocol, processes also send auxiliary messages that inform peers about the
executed steps of the core consensus algorithm (`NewRoundStepMessage` and `NewValidBlockMessage`), and
also messages that inform peers what votes the process has seen (`HasVoteMessage`,
`VoteSetMaj23Message` and `VoteSetBitsMessage`). These messages are then used in the gossiping
protocol to determine what messages a process should send to its peers.

We now describe the content of each message exchanged during Tendermint consensus protocol.

## ProposalMessage

ProposalMessage is sent when a new block is proposed. It is a suggestion of what the
next block in the blockchain should be.

```go
type ProposalMessage struct {
    Proposal Proposal
}
```

### Proposal

Proposal contains height and round for which this proposal is made, BlockID as a unique identifier
of proposed block, timestamp, and POLRound (a so-called Proof-of-Lock (POL) round) that is needed for
termination of the consensus. If POLRound >= 0, then BlockID corresponds to the block that 
is locked in POLRound. The message is signed by the validator private key.

```go
type Proposal struct {
    Height           int64
    Round            int
    POLRound         int
    BlockID          BlockID
    Timestamp        Time
    Signature        Signature
}
```

## VoteMessage

VoteMessage is sent to vote for some block (or to inform others that a process does not vote in the
current round). Vote is defined in the [Blockchain](https://github.com/tendermint/tendermint/blob/master/docs/spec/blockchain/blockchain.md#blockid) section and contains validator's
information (validator address and index), height and round for which the vote is sent, vote type,
blockID if process vote for some block (`nil` otherwise) and a timestamp when the vote is sent. The
message is signed by the validator private key.

```go
type VoteMessage struct {
    Vote Vote
}
```

## BlockPartMessage

BlockPartMessage is sent when gossipping a piece of the proposed block. It contains height, round
and the block part.

```go
type BlockPartMessage struct {
    Height int64
    Round  int
    Part   Part
}
```

## NewRoundStepMessage

NewRoundStepMessage is sent for every step transition during the core consensus algorithm execution.
It is used in the gossip part of the Tendermint protocol to inform peers about a current
height/round/step a process is in.

```go
type NewRoundStepMessage struct {
    Height                int64
    Round                 int
    Step                  RoundStepType
    SecondsSinceStartTime int
    LastCommitRound       int
}
```

## NewValidBlockMessage

NewValidBlockMessage is sent when a validator observes a valid block B in some round r, 
i.e., there is a Proposal for block B and 2/3+ prevotes for the block B in the round r.
It contains height and round in which valid block is observed, block parts header that describes 
the valid block and is used to obtain all
block parts, and a bit array of the block parts a process currently has, so its peers can know what
parts it is missing so they can send them.
In case the block is also committed, then IsCommit flag is set to true.

```go
type NewValidBlockMessage struct {
    Height           int64
    Round            int    
    BlockPartsHeader PartSetHeader
    BlockParts       BitArray
    IsCommit         bool
}
```

## ProposalPOLMessage

ProposalPOLMessage is sent when a previous block is re-proposed.
It is used to inform peers in what round the process learned for this block (ProposalPOLRound),
and what prevotes for the re-proposed block the process has.

```go
type ProposalPOLMessage struct {
    Height           int64
    ProposalPOLRound int
    ProposalPOL      BitArray
}
```

## HasVoteMessage

HasVoteMessage is sent to indicate that a particular vote has been received. It contains height,
round, vote type and the index of the validator that is the originator of the corresponding vote.

```go
type HasVoteMessage struct {
    Height int64
    Round  int
    Type   byte
    Index  int
}
```

## VoteSetMaj23Message

VoteSetMaj23Message is sent to indicate that a process has seen +2/3 votes for some BlockID.
It contains height, round, vote type and the BlockID.

```go
type VoteSetMaj23Message struct {
    Height  int64
    Round   int
    Type    byte
    BlockID BlockID
}
```

## VoteSetBitsMessage

VoteSetBitsMessage is sent to communicate the bit-array of votes a process has seen for a given
BlockID. It contains height, round, vote type, BlockID and a bit array of
the votes a process has.

```go
type VoteSetBitsMessage struct {
    Height  int64
    Round   int
    Type    byte
    BlockID BlockID
    Votes   BitArray
}
```
