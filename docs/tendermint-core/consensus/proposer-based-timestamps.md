---
order: 3
---

# PBTS

 This document provides an overview of the Proposer-Based Timestamp (PBTS)
 algorithm added to Tendermint in the v0.36 release. It outlines the core
 functionality as well as the parameters and constraints of the this algorithm.

## Algorithm Overview

The PBTS algorithm defines a way for a Tendermint blockchain to create block
timestamps that are within a reasonable bound of the clocks of the validators on
the network. This replaces the original BFTTime algorithm for timestamp
assignment that computed a timestamp using the timestamps included in precommit
messages.

## Algorithm Parameters

The functionality of the PBTS algorithm is governed by two parameters within
Tendermint. These two parameters are [consensus
parameters](https://github.com/tendermint/tendermint/blob/master/spec/abci/apps.md#L291),
meaning they are configured by the ABCI application and are therefore the same
same across all nodes on the network.

### `Precision`

The `Precision` parameter configures the acceptable upper-bound of clock drift
among all of the nodes on a Tendermint network. Any two nodes on a Tendermint
network are expected to have clocks that differ by at most `Precision`
milliseconds any given instant.

### `MessageDelay`

The `MessageDelay` parameter configures the acceptable upper-bound for
transmitting a `Proposal` message from the proposer to _all_ of the validators
on the network.

Networks should choose as small a value for `MessageDelay` as is practical,
provided it is large enough that messages can reach all participants with high
probability given the number of participants and latency of their connections.

## Algorithm Concepts

### Block timestamps

Each block produced by the Tendermint consensus engine contains a timestamp.
The timestamp produced in each block is a meaningful representation of time that is
useful for the protocols and applications built on top of Tendermint.

The following protocols and application features require a reliable source of time:

* Tendermint Light Clients [rely on correspondence between their known time](https://github.com/tendermint/tendermint/blob/master/spec/light-client/verification/README.md#definitions-1) and the block time for block verification.
* Tendermint Evidence expiration is determined [either in terms of heights or in terms of time](https://github.com/tendermint/tendermint/blob/master/spec/consensus/evidence.md#verification).
* Unbonding of staked assets in the Cosmos Hub [occurs after a period of 21
 days](https://github.com/cosmos/governance/blob/master/params-change/Staking.md#unbondingtime).
* IBC packets can use either a [timestamp or a height to timeout packet
 delivery](https://docs.cosmos.network/v0.44/ibc/overview.html#acknowledgements)

### Proposer Selects a Block Timestamp

When the proposer node creates a new block proposal, the node reads the time
from its local clock and uses this reading as the timestamp for the proposed
block.

### Timeliness

When each validator on a Tendermint network receives a proposed block, it
performs a series of checks to ensure that the block can be considered valid as
a candidate to be the next block in the chain.

The PBTS algorithm performs a validity check on the timestamp of proposed
blocks. When a validator receives a proposal it ensures that the timestamp in
the proposal is within a bound of the validator's local clock. Specifically, the
algorithm checks that the timestamp is no more than `Precision` greater than the
node's local clock and no less than `Precision` + `MessageDelay` behind than the
node's local clock. This creates range of acceptable timestamps around the
node's local time. If the timestamp is within this range, the PBTS algorithm
considers the block **timely**. If a block is not **timely**, the node will
issue a `nil` `prevote` for this block, signaling to the rest of the network
that the node does not consider the block to be valid.

### Clock Synchronization

The PBTS algorithm requires the clocks of the validators on a Tendermint network
are within `Precision` of each other. In practice, this means that validators
should periodically synchronize to a reliable NTP server. Validators that drift
too far away from the rest of the network will no longer propose blocks with
valid timestamps. Additionally they will not view the timestamps of blocks
proposed by their peers to be valid either.

## See Also

* [The PBTS specification](https://github.com/tendermint/tendermint/blob/master/spec/consensus/proposer-based-timestamp/README.md)
 contains all of the details of the algorithm.
