# ADR 065: Custom Event Indexing

## Changelog

- April 1, 2021: Initial Draft (@alexanderbez)

## Status

Proposed

## Context

Currently, Tendermint Core supports block and transaction event indexing through
the `tx_index.indexer` configuration. Events are captured in transactions and
are indexed via a `TxIndexer` type. Events are captured in blocks, specifically
from `BeginBlock` and `EndBlock` application responses, and are indexed via a
`BlockIndexer` type. Both of these types are managed by a single `IndexerService`
which is responsibly for consuming events and sending those events off to be
indexed by the respective type.

In addition to indexing, Tendermint Core also supports the ability to query for
both indexed transaction and block events via Tendermint's RPC layer. The ability
to query for these indexed events facilitates a great multitude of upstream client
and application capabilities, e.g. block explorers, IBC relayers, and auxiliary
data availability and indexing services.

Currently, Tendermint only supports indexing via a `kv` indexer, which is supported
by an underlying embedded key/value store database. The `kv` indexer implements
its own indexing and query mechanisms. While the former is somewhat trivial,
providing a rich and flexible query layer is not as trivial and has caused many
issues and UX concerns for upstream clients and applications.

The fragile nature of the proprietary `kv` query engine and the potential
performance and scaling issues that arise when a large number of consumers are
introduced, motivate the need for a more robust and flexible indexing and query
solution.

## Alternative Approaches

> This section contains information around alternative options that are considered
> before making a decision. It should contain a explanation on why the alternative
> approach(es) were not chosen.

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

> Are there any relevant PR comments, issues that led up to this, or articles
> referenced for why we made the given design choice? If so link them here!

- {reference link}
