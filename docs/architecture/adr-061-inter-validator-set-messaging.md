# ADR 061: Inter validator set messaging

## Changelog

- 2021-09-29: Initial version of the document

## Context

### Definitions

For the needs of this document, we define the following types of nodes:

- Validator nodes, which participate in the consensus protocol, verify and sign blocks,
- Full nodes, which store all blocks from the blockchain.

Note: there are also light nodes, but light nodes are not relevant for the discussion.

Validator Set is a group of Validator nodes responsible for the consensus at a given time. Only one Validator Set can
be active at the given time. After a predefined number of blocks, the ABCI app initiates the Validator Set rotation to
make another Validator Set active. 

### Problem statement

The consensus process requires direct communication between validators, as full nodes do not accept nor propagate consensus protocol messages (like votes).

Every node, regardless of its type, selects peer nodes they connect to on a random basis. There is no differentiation
between full and Validator nodes, and there is no method to ensure that each Validator is (directly or through other
validators) connected to all other validators. In an extreme case, a validator node can be connected only to full
nodes. It will block its communication and effectively exclude it from participation in the consensus.

### Solution

Each Validator node shall allocate a preconfigured number of connectivity slots for communication with other Validators
that belong to the same Validator Set.

ABCI application manages the rotation of Validator Sets. When rotation is needed, the application sends information about the new Validator Set in response to the `EndBlock` message. Each member of the active Validator Set establishes connections with a predefined number of Peer Validators belonging to the same Validator Set. A list of Peer Validators is determined using the algorithm described in [DIP 0006](https://github.com/dashpay/dips/blob/master/dip-0006.md#building-the-set-of-deterministic-connections).

## Alternative Approaches

> TODO: This section contains information around alternative options that are considered before making a decision.
> It should contain a explanation on why the alternative approach(es) were not chosen.

## Decision

> TODO: This section records the decision that was made.
> It is best to record as much info as possible from the discussion that happened. This aids in not having to go back
> to the Pull Request to get the needed information.

## Detailed Design

> TODO: This section does not need to be filled in at the start of the ADR, but must be completed prior to the merging
> of the implementation.
>
> Here are some common questions that get answered as part of the detailed design:
>
> - What are the user requirements?
>
> - What systems will be affected?
>
> - What new data structures are needed, what data structures will be changed?
>
> - What new APIs will be needed, what APIs will be changed?
>
> - What are the efficiency considerations (time/space)?
>
> - What are the expected access patterns (load/throughput)?
>
> - Are there any logging, monitoring or observability needs?
>
> - Are there any security considerations?
>
> - Are there any privacy considerations?
>
> - How will the changes be tested?
>
> - If the change is large, how will the changes be broken up for ease of review?
>
> - Will these changes require a breaking (major) release?
>
> - Does this change require coordination with the SDK or other?

## Status

> TODO: A decision may be "proposed" if it hasn't been agreed upon yet, or "accepted" once it is agreed upon. 
> Once the ADR has been implemented mark the ADR as "implemented". If a later ADR changes or reverses a decision, 
> it may be marked as "deprecated" or "superseded" with a reference to its replacement.

{Deprecated|Proposed|Accepted|Declined}

## Consequences

> TODO: This section describes the consequences, after applying the decision. All consequences should be summarized 
> here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References

> TODO: Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given
> design choice? If so link them here!

- [Dash Core Group Release Announcement: Dash Platform v0.20 on Testnet](https://blog.dash.org/dash-core-group-release-announcement-dash-platform-v0-20-on-testnet-c8fa00d28af7)
- [DIP 0006: Long Living Masternode Quorums (LLMQ)](https://github.com/dashpay/dips/blob/master/dip-0006.md)
- {reference link}
