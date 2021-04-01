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

With regards to alternative approaches to a more robust solution, the only serious
contender that was considered was to transition to using [SQLite](https://www.sqlite.org/index.html).

While the approach would work, it locks us into a specific query language and
storage layer, so in some ways it's only a bit better than our current approach.
In addition, the implementation would require the introduction of CGO into the
Tendermint Core stack, whereas right now CGO is only introduced depending on
the database used.

## Decision

We will adopt a similar approach to that of the Cosmos SDK's `KVStore` state
listening described in [ADR-038](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-038-state-listening.md).

Namely, we will perform the following:

- Introduce a new interface, `EventSink`, that all data sinks must implement.
- Augment the existing `tx_index.indexer` configuration to now accept a series
  of one or more indexer types, i.e sinks.
- Combine the current `TxIndexer` and `BlockIndexer` into a single `KVEventSink`
  that implements the `EventSink` interface.
- Introduce an additional `EventSink` that is backed by [PostgreSQL](https://www.postgresql.org/).
  - Implement the necessary schemas to support both block and transaction event
  indexing.
- Update `IndexerService` to use a series of `EventSinks`.
- Proxy queries to the relevant sink's native query layer.
- Update all relevant RPC methods.


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

### Positive

- A more robust and flexible indexing and query engine for indexing and search
  block and transaction events.
- The ability to not have to support a custom indexing and query engine beyond
  the legacy `kv` type.

### Negative

- The need to support multiple and potentially a growing set of custom `EventSink`
  types.

### Neutral

## References

- [Cosmos SDK ADR-038](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-038-state-listening.md)
