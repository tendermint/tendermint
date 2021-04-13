# ADR 065: Custom Event Indexing

- [ADR 065: Custom Event Indexing](#adr-065-custom-event-indexing)
  - [Changelog](#changelog)
  - [Status](#status)
  - [Context](#context)
  - [Alternative Approaches](#alternative-approaches)
  - [Decision](#decision)
  - [Detailed Design](#detailed-design)
    - [EventSink](#eventsink)
    - [Supported Sinks](#supported-sinks)
      - [`KVEventSink`](#kveventsink)
      - [`PSQLEventSink`](#psqleventsink)
    - [Configuration](#configuration)
  - [Future Improvements](#future-improvements)
  - [Consequences](#consequences)
    - [Positive](#positive)
    - [Negative](#negative)
    - [Neutral](#neutral)
  - [References](#references)

## Changelog

- April 1, 2021: Initial Draft (@alexanderbez)

## Status

Accepted

## Context

Currently, Tendermint Core supports block and transaction event indexing through
the `tx_index.indexer` configuration. Events are captured in transactions and
are indexed via a `TxIndexer` type. Events are captured in blocks, specifically
from `BeginBlock` and `EndBlock` application responses, and are indexed via a
`BlockIndexer` type. Both of these types are managed by a single `IndexerService`
which is responsible for consuming events and sending those events off to be
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

#### `KVEventSink`

This type of `EventSink` is a combination of the  `TxIndexer` and `BlockIndexer`
indexers, both of which are backed by a single embedded key/value database.

A bulk of the existing business logic will remain the same, but the existing APIs
mapped to the new `EventSink` API. Both types will be removed in favor of a single
`KVEventSink` type.

The `KVEventSink` will be the only `EventSink` enabled by default, so from a UX
perspective, operators should not notice a difference apart from a configuration
change.

We omit `EventSink` implementation details as it should be fairly straightforward
to map the existing business logic to the new APIs.

#### `PSQLEventSink`

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
    type block_event_type
);

CREATE TABLE IF NOT EXISTS tx_results {
  id SERIAL PRIMARY KEY,
  tx_result BYTEA NOT NULL
}

CREATE TABLE IF NOT EXISTS tx_events (
    id SERIAL PRIMARY KEY,
    key VARCHAR NOT NULL,
    value VARCHAR NOT NULL,
    height INTEGER NOT NULL,
    hash VARCHAR NOT NULL,
    FOREIGN KEY (tx_result_id) REFERENCES tx_results(id) ON DELETE CASCADE
);

-- Indices -------------------------------------------------------

CREATE INDEX idx_block_events_key_value ON block_events(key, value);
CREATE INDEX idx_tx_events_key_value ON tx_events(key, value);
CREATE INDEX idx_tx_events_hash ON tx_events(hash);
```

The `PSQLEventSink` will implement the `EventSink` interface as follows
(some details omitted for brevity):


```go
func NewPSQLEventSink(connStr string) (*PSQLEventSink, error) {
  db, err := sql.Open("postgres", connStr)
  if err != nil {
    return nil, err
  }

  // ...
}

func (es *PSQLEventSink) IndexBlockEvents(h types.EventDataNewBlockHeader) error {
  sqlStmt := sq.Insert("block_events").Columns("key", "value", "height", "type")

  // index the reserved block height index
  sqlStmt = sqlStmt.Values(types.BlockHeightKey, h.Header.Height, h.Header.Height, "")

  for _, event := range h.ResultBeginBlock.Events {
    // only index events with a non-empty type
    if len(event.Type) == 0 {
      continue
    }

    for _, attr := range event.Attributes {
      if len(attr.Key) == 0 {
        continue
      }

      // index iff the event specified index:true and it's not a reserved event
      compositeKey := fmt.Sprintf("%s.%s", event.Type, string(attr.Key))
      if compositeKey == types.BlockHeightKey {
        return fmt.Errorf("event type and attribute key \"%s\" is reserved; please use a different key", compositeKey)
      }

      if attr.GetIndex() {
        sqlStmt = sqlStmt.Values(compositeKey, string(attr.Value), h.Header.Height, BlockEventTypeBeginBlock)
      }
    }
  }

  // index end_block events...
  // execute sqlStmt db query...
}

func (es *PSQLEventSink) IndexTxEvents(txr *abci.TxResult) error {
  sqlStmtEvents := sq.Insert("tx_events").Columns("key", "value", "height", "hash", "tx_result_id")
  sqlStmtTxResult := sq.Insert("tx_results").Columns("tx_result")


  // store the tx result
  txBz, err := proto.Marshal(txr)
  if err != nil {
    return err
  }

  sqlStmtTxResult = sqlStmtTxResult.Values(txBz)

  // execute sqlStmtTxResult db query...

  // index the reserved height and hash indices
  hash := types.Tx(txr.Tx).Hash()
  sqlStmtEvents = sqlStmtEvents.Values(types.TxHashKey, hash, txr.Height, hash, txrID)
  sqlStmtEvents = sqlStmtEvents.Values(types.TxHeightKey, txr.Height, txr.Height, hash, txrID)

  for _, event := range result.Result.Events {
    // only index events with a non-empty type
    if len(event.Type) == 0 {
      continue
    }

    for _, attr := range event.Attributes {
      if len(attr.Key) == 0 {
        continue
      }

      // index if `index: true` is set
      compositeTag := fmt.Sprintf("%s.%s", event.Type, string(attr.Key))
			
      // ensure event does not conflict with a reserved prefix key
      if compositeTag == types.TxHashKey || compositeTag == types.TxHeightKey {
        return fmt.Errorf("event type and attribute key \"%s\" is reserved; please use a different key", compositeTag)
      }
		
      if attr.GetIndex() {
        sqlStmtEvents = sqlStmtEvents.Values(compositeKey, string(attr.Value), txr.Height, hash, txrID)
      }
    }
  }

  // execute sqlStmtEvents db query...
}

func (es *PSQLEventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
  sqlStmt = sq.Select("height").Distinct().From("block_events").Where(q.String())

  // execute sqlStmt db query and scan into integer rows...
}

func (es *PSQLEventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
  sqlStmt = sq.Select("tx_result_id").Distinct().From("tx_events").Where(q.String())

  // execute sqlStmt db query and scan into integer rows...
  // query tx_results records and scan into binary slice rows...
  // decode each row into a TxResult...
}
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

Any invalid or misconfigured `tx_index` configuration should yield an error as
early as possible.

## Future Improvements

Although not technically required to maintain feature parity with the current
existing Tendermint indexer, it would be beneficial for operators to have a method
of performing a "re-index". Specifically, Tendermint operators could invoke an
RPC method that allows the Tendermint node to perform a re-indexing of all block
and transaction events between two given heights, H<sub>1</sub> and H<sub>2</sub>,
so long as the block store contains the blocks and transaction results for all
the heights specified in a given range.

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
