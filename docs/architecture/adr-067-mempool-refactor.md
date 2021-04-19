# ADR 067: Mempool Refactor


## Changelog

- April 19, 2021: Initial Draft (@alexanderbez)

## Status

Proposed

## Context

Tendermint Core has a reactor and data structure, mempool, that facilitates the
ephemeral storage of uncommitted transactions. Honest nodes participating in a
Tendermint network gossip these uncommitted transactions to each other if they
pass the application's `CheckTx`. In addition, block proposers select from the
mempool a subset of uncommitted transactions to include in the next block.

Currently, the mempool in Tendermint Core is designed as a FIFO queue. In other
words, transactions are included in blocks as they are received by a node. There
currently is no explicit and prioritized ordering of these uncommitted transactions.
This presents a few technical and UX challenges for operators and applications.

Namely, validators are not able to prioritize transactions by their fees or any
incentive aligned mechanism. In addition, the lack of prioritization also leads
to cascading effects in terms of DoS and various attack vectors on networks,
e.g. [cosmos/cosmos-sdk#8224](https://github.com/cosmos/cosmos-sdk/discussions/8224).

Thus, Tendermint Core needs the ability for an application and its users to
prioritize transactions in a flexible and performant manner.

## Alternative Approaches

When considering which approach to take for a priority-based flexible and
performant mempool, there are two core candidates. The first candidate in less
invasive in the required  set of protocol and implementation changes, which
simply extends the existing `CheckTx` ABCI method. The second candidate essentially
involves the introduction of new ABCI method(s) and would require a higher degree
of complexity in protocol and implementation changes, some of which may either
overlap or conflict with the upcoming introduction of [ABCI++](https://github.com/tendermint/spec/blob/master/rfc/004-abci%2B%2B.md).

For more information on the various approaches and proposals, please see the
[mempool discussion](https://github.com/tendermint/tendermint/discussions/6295).

## Decision

> This section records the decision that was made.
> It is best to record as much info as possible from the discussion that happened.
> This aids in not having to go back to the Pull Request to get the needed information.

## Detailed Design

> This section does not need to be filled in at the start of the ADR, but must
> be completed prior to the merging of the implementation.
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

## Consequences

> This section describes the consequences, after applying the decision. All
> consequences should be summarized here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References
