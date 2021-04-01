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

### EventSink

We introduce the `EventSink` interface type that all supported sinks must implement.
The interface is defined as follows:

```go
type EventSink interface {
  IndexBlockEvents(types.EventDataNewBlockHeader) error
  IndexTxEvents(*abci.TxResult) error

  SearchBlockEvents(context.Context, *query.Query) ([]int64, error)
  SearchTxEvents(context.Context, *query.Query) ([]*abci.TxResult, error)

  GetTxByHash([]byte) (*abci.TxResult, error)
  HasBlock(int64) (bool, error)
}
```

The `IndexerService`  will accept a list of one or more `EventSink` types. During
the `OnStart` method it will call the appropriate APIs on each `EventSink` to
index both block and transaction events.

### Supported Sinks

We will initially support two `EventSink` types out of the box.

1. `KVEventSink`

This type of `EventSink` is a combination of the  `TxIndexer` and `BlockIndexer`
indexers, both of which are backed by a single embedded key/value database.

A bulk of the existing business logic will remain the same, but the existing APIs
mapped to the new `EventSink` API. Both types will be removed in favor of a single
`KVEventSink` type.

The `KVEventSink` will be the only `EventSink` enabled by default, so from a UX
perspective, operators should not notice a difference apart from a configuration
change.

2. `PSQLEventSink`

This type of `EventSink` indexes block and transaction events into a [PostgreSQL](https://www.postgresql.org/).
database. We define and automatically migrate the following schema when the
`IndexerService` starts.

```sql
-- Table Definition ----------------------------------------------

CREATE TYPE IF NOT EXISTS block_event_type AS ENUM ('begin_block', 'end_block');

CREATE TABLE IF NOT EXISTS block_events (
    id SERIAL PRIMARY KEY,
    key VARCHAR NOT NULL,
    value VARCHAR NOT NULL,
    height INTEGER NOT NULL,
    type block_event_type NOT NULL
);

CREATE TABLE IF NOT EXISTS tx_events (
    id SERIAL PRIMARY KEY,
    key VARCHAR NOT NULL,
    value VARCHAR NOT NULL,
    height INTEGER NOT NULL,
    hash VARCHAR NOT NULL
);

-- Indices -------------------------------------------------------

CREATE INDEX idx_block_events_key_value ON block_events(key, value);
CREATE INDEX idx_tx_events_key_value ON tx_events(key, value);
CREATE INDEX idx_tx_events_hash ON tx_events(hash);
```
### Configuration

The current `tx_index.indexer` configuration would be changed to accept a list
of supported `EventSink` types instead of a single value.

Example:

```toml
[tx_index]

indexer = [
  "kv",
  "psql"
]
```

If the `indexer` list contains the `null` indexer, then no indexers will be used
regardless of what other values may exist.

Additional configuration parameters might be required depending on what event
sinks are supplied to `tx_index.indexer`. The `psql` will require an additional
connection configuration.

```toml
[tx_index]

indexer = [
  "kv",
  "psql"
]

pqsql_conn = "postgresql://<user>:<password>@<host>:<port>/<db>?<opts>"
```

### Node

### RPC


## Consequences

### Positive

- A more robust and flexible indexing and query engine for indexing and search
  block and transaction events.
- The ability to not have to support a custom indexing and query engine beyond
  the legacy `kv` type.
- The ability to offload/proxy indexing and querying to the underling sink.
- Scalability and reliability that essentially comes "for free" from the underlying
  sink, if it supports it.

### Negative

- The need to support multiple and potentially a growing set of custom `EventSink`
  types.

### Neutral

## References

- [Cosmos SDK ADR-038](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-038-state-listening.md)
- [PostgreSQL](https://www.postgresql.org/)
- [SQLite](https://www.sqlite.org/index.html)
