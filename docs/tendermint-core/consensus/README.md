---
order: 1
parent:
  title: Consensus
  order: 6
---

# Consensus

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
[Blockchain](https://github.com/tendermint/tendermint/blob/master/spec/core/data_structures.md#blockid) section)
that uniquely identifies each block. The block itself is
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
