# ADR 082: Data Companion API

## Changelog

- 2022-09-10: First draft (@thanethomson)

## Status

Accepted | Rejected | Deprecated | Superseded by

## Context

This ADR proposes to effectively offload some data storage responsibilities and
functionality to **single external companion service** in order to:

1. Improve core consensus stability.
2. Eventually reduce the amount of functionality for which the core team is
   responsible, thereby improving maintainability.
3. Still cater for certain use cases that require this data and functionality.

The way the system is currently built is such that a Tendermint node is mostly
self-contained. While this philosophy initially allowed a certain degree of ease
of node operation (i.e. simple UX), it has also lent itself to feature sprawl,
with Tendermint being asked to take care of increasingly more than just
consensus. Under the spotlight in this ADR are:

1. **Event indexing**, which is required in order to facilitate arbitrary block
   and transaction querying, as well as subscription for arbitrary events. We
   have already seen, for instance, how the current (somewhat unreliable) event
   subscription implementation can cause back-pressure into consensus and affect
   IBC relayer stability (see [\#6729] and [\#7156]).
2. **Block execution result storage**, which is required to facilitate a number
   of RPC APIs, but also results in storage of large quantities of data that is
   not critical to consensus. This data is, however, critical for certain use
   cases outside of consensus.

Another intersecting issue is that there may be a variety of use cases that
require different data models. A good example of this is the mere existence of
the [PostgreSQL indexer] as compared to the default key/value indexer.

Continuing from and expanding on the ideas outlined in [RFC 006][rfc-006], the
suggested approach in this ADR is to provide a mechanism to publish certain data
from Tendermint, in real-time and with certain reliability guarantees, to a
single companion service outside of the node that can use this data in whatever
way it chooses (filter and republish it, store it, manipulate or enrich it,
etc.).

Specifically, this mechanism would initially publish:

1. The results of block execution (e.g. data from `FinalizeBlockResponse`). This
   data is not accessible from the P2P layer, and currently provides valuable
   information for Tendermint users (whether events should be handled at all by
   Tendermint is a different problem).
2. All block data (i.e. `Block` data structures that have been committed by the
   consensus engine).

## Alternative Approaches

1. One clear alternative to this would be the approach outlined in [ADR
   075][adr-075]. This approach:

   1. Still leaves Tendermint responsible for maintaining a query interface and
      event indexing functionality, increasing the long-term maintenance burden of,
      and the possibility of feature sprawl in, that subsystem. To overcome this,
      we could remove the query interface altogether and just always publish all
      data.
   2. Only keeps track of a sliding window of events (and does so in-memory), and
      so does not provide reliable guaranteed delivery of this data. Even just
      persisting this data will not solve this problem: if we prioritize
      continued operation of the node over guaranteed delivery of this data, we
      may eventually run out of disk storage. So here we would need to persist the
      data _and_ we would need to panic if the sliding window of events fills up.
   3. Allows for multiple consumers. While desirable from typical web services,
      Tendermint is a consensus engine and, as such, we should prioritize consensus
      over providing similar guarantees to a scalable RPC interface (especially
      since we know this does not scale, and if such a job could just be
      outsourced). If we have a single consumer, we can easily automatically prune
      data that was already sent. We could overcome this by limiting the API to a
      single consumer and single event window.

   Making the requisite modifications to ADR-075 basically converges on the
   solution outlined in this ADR, except that ADR-075 publishes data via JSON over
   HTTP while this solution proposes gRPC.

2. We could pick a database that provides real-time notifications of inserts to
   consumers and just dump the requisite data into that database. Consumers
   would then need to connect to that database, listen for updates, and then
   transform/process the data as needed.

   This approach could also inevitably lead to feature sprawl in people wanting
   us to support multiple different databases. It is also not clear which
   database would be the best option here.

## Decision

> TODO(thane)

## Detailed Design

### Use Cases

1. Transaction facilitators could provide standalone RPC services that would be
   capable of receiving transactions and providing confirmations to submitters.
   These RPC services could be architected to scale independently of the node
   facilitating consensus.
2. Block explorers could make use of this data to provide real-time information
   about the blockchain.
3. IBC relayer nodes could use this data to filter and store only the data they
   need to facilitate relaying without putting additional pressure on the
   consensus engine (until such time that a decision is made on whether to
   continue providing event data from Tendermint).

### Requirements

1. All of the following data must be published to a single external consumer:
   1. `FinalizeBlockResponse` data, which includes at the very least:
      1. Events
      2. Transaction results
   2. Committed block data

2. It must be opt-in. In other words, it is off by default and turned on by way
   of a configuration parameter. When off, it must have negligible (ideally no)
   impact on system performance.

3. It must not cause back-pressure into consensus.

4. It must not cause unbounded memory growth.

5. Data must be published reliably. If data cannot be published, the node must
   rather crash in such a way that bringing the node up again will cause it to
   attempt to republish the unpublished data.

6. The interface must support extension by allowing more kinds of data to be
   exposed via this API (bearing in mind that every extension has the potential
   to affect system stability in the case of an unreliable data companion
   service).

### Entity Relationships

The following simple diagram shows the proposed relationships between
Tendermint, a socket-based ABCI application, and the proposed data companion
service to contextualize this ADR.

```
     +----------+      +------------+      +----------------+
     | ABCI App | <--- | Tendermint | ---> | Data Companion |
     +----------+      +------------+      +----------------+

```

As can be seen in this diagram, Tendermint connects out to both the ABCI
application and data companion service based on the Tendermint node's
configuration. The fact that Tendermint connects out to the companion service
instead of the other way around provides a natural constraint on the number of
consumers of the API.

### gRPC API

The following gRPC API is proposed:

```protobuf
// DataCompanionService allows Tendermint to publish certain data generated by
// the consensus engine to a single external consumer with specific reliability
// guarantees.
//
// Note that implementers of this service must take into account the possibility
// that Tendermint may re-send data that was previously sent. Therefore
// the service should simply ignore previously seen data instead of responding
// with errors to ensure correct functioning of the node.
service DataCompanionService {
    // CommitBlock is called after a block has been committed. This method is
    // also called on Tendermint startup to ensure that the service received the
    // last committed block, in case there was previously a transport failure.
    //
    // If an error is returned, Tendermint will crash.
    rpc CommitBlock (CommitBlockRequest) returns (CommitBlockResponse) {}
}

// CommitBlockRequest contains information about a block that has been committed
// by the consensus engine.
message CommitBlockRequest {
    // The block committed by the consensus engine.
    Block block = 1;

    // The results from execution of this block.
    ExecBlockResults block_results = 2;
}

// ExecBlockResults contains additional information about the execution of a
// specific block that will not necessarily be included in the block itself.
// This data is obtained from the FinalizeBlock ABCI response.
message ExecBlockResults {
    repeated Event events = 1;
    repeated ExecTxResult tx_results = 2;
}

// CommitBlockResponse is either empty or returns an error. Note that returning
// an error here will cause Tendermint to crash.
message CommitBlockResponse {
    oneof error {
        UnexpectedBlockError unexpected_block_err = 1;
    }
}

// UnexpectedBlockError is an error returned by the server when Tendermint sent
// it a block that is ahead of the block expected by the server.
message UnexpectedBlockError {
    // The height of the block expected by the server.
    int64 expected_height = 1;
}

//-----------------------------------------------------------------------------
// The following types are defined as part of ABCI++, and provided here for
// reference.
//-----------------------------------------------------------------------------

// Event allows application developers to attach additional information to
// ResponseFinalizeBlock, ResponseDeliverTx, ExecTxResult
// Later, transactions may be queried using these events.
message Event {
  string                  type       = 1;
  repeated EventAttribute attributes = 2 [(gogoproto.nullable) = false, (gogoproto.jsontag) = "attributes,omitempty"];
}

// EventAttribute is a single key-value pair, associated with an event.
message EventAttribute {
  string key   = 1;
  string value = 2;
  bool   index = 3;  // nondeterministic
}

// ExecTxResult contains results of executing one individual transaction.
//
// * Its structure is equivalent to #ResponseDeliverTx which will be deprecated/deleted
message ExecTxResult {
  uint32         code       = 1;
  bytes          data       = 2;
  string         log        = 3;  // nondeterministic
  string         info       = 4;  // nondeterministic
  int64          gas_wanted = 5;
  int64          gas_used   = 6;
  repeated Event events     = 7
      [(gogoproto.nullable) = false, (gogoproto.jsontag) = "events,omitempty"];  // nondeterministic
  string codespace = 8;
}
```

### Request Buffering

In order to ensure reliable delivery, while also catering for intermittent
faults, we should allow for configurable buffering of a reasonable, yet small,
number of requests to the companion service. If this buffer fills up, we should
panic.

This data would need to be stored on disk to ensure that Tendermint could
attempt to resubmit all unsubmitted data upon Tendermint startup. At present,
this data is already stored on disk, and so practically we would need to
implement some form of background pruning mechanism to remove the data we know
has been shared with the companion service.

### Interaction with Block Sync

During block sync, request buffering should be disabled and requests to the
companion service should be blocking.

### Configuration

The following configuration file update is proposed to support the data
companion API.

```toml
[data_companion]
# By default, the data companion service interaction is disabled. It is
# recommended that this only be enabled on full nodes and not validators so as
# to minimize the possibility of network instability.
enabled = false

# Address at which the gRPC companion service server is hosted. It is
# recommended that this companion service be co-located at least within the same
# data center as the Tendermint node to reduce the risk of network latencies
# interfering in node operation.
addr = "http://localhost:26659"

# The number of requests to the companion service to durably buffer. Set to 0 to
# enable blocking send mode, which will block on every request to the companion
# service. A sensible non-zero value would be 10 (if the companion service is
# unavailable to receive 10 blocks' data, something is probably wrong and
# requires intervention).
buffer_size = 0

# Use the experimental gzip compression call option when submitting data to the
# server. See https://pkg.go.dev/google.golang.org/grpc#UseCompressor
experimental_use_gzip = false
```

It is unclear at present whether compressing the data being sent to the
companion service will result in meaningful benefits.

### Monitoring

To monitor the health of the interaction between Tendermint and the companion
service, the following additional Prometheus metrics are proposed:

- `data_companion_send_time` - A gauge that indicates the maximum time, in
  milliseconds, taken to send a single request to the companion service take
  from a rolling window of tracked send times (e.g. the maximum send time over
  the past minute).
- `data_companion_buffer_utilization` - A gauge indicating how much of the
  request buffer has been used.

## Implications

1. We will be able to mark the following RPC APIs as deprecated and schedule
   them for removal in a future release:
   1. The [WebSocket subscription API][websocket-api]
   2. [`/tx_search`]
   3. [`/block_search`]
   4. [`/broadcast_tx_commit`]
   5. [`/block_results`]

2. We will be able to remove all event indexing from Tendermint once we remove
   the above APIs.

3. Depending on the implementation approach chosen, we will still need to store
   some quantity of data not critical to consensus. This data can automatically
   be pruned once Tendermint has successfully transmitted it to the companion
   service.

### Release Strategy

As captured in the [requirements](#requirements), we should be able to release
this as an additive, opt-in change, thereby not impacting existing APIs. This
way we can evolve this API until such time that consumers are satisfied with
removal of the old APIs mentioned in [implications](#implications).

This also means we could potentially release this as a non-breaking change (i.e.
in a patch release).

## Follow-Up Work

If implemented, we should consider releasing our own RPC companion server with
easy deployment instructions for people to test it out and compare the
performance with directly connecting to the Tendermint node.

This RPC companion server could easily implement an interface like that outlined
in [ADR-075](adr-075), providing the same HTTP- and cursor-based subscription
mechanism, but storing data to and pulling data from a different database (e.g.
PostgreSQL).

## Consequences

### Positive

- Paves the way toward greater architectural simplification of Tendermint so it
  can focus on its core duty, consensus, while still facilitating existing use
  cases.
- Allows us to eventually separate out non-operator-focused RPC functionality
  from Tendermint entirely, allowing the RPC to scale independently of the
  consensus engine.
- Can be rolled out as experimental and opt-in in a non-breaking way.
- The broad nature of what the API publishes lends itself to reasonable
  long-term stability.

### Negative

- Keeping existing Tendermint functionality would involve operators having to
  run an additional process, which increases operational complexity.
- It is unclear at present as to the impact of the requirement to publish large
  quantities of block/result data on the speed of block execution. This should
  be quantified in production networks as soon as this feature can be rolled out
  as experimental. If the impact is meaningful, we should either remove the
  feature or develop mitigation strategies (e.g. allowing for queries to be
  specified via the configuration file, or supporting a specific subset of use
  cases' data).
- Requires a reasonable amount of coordination work across a number of
  stakeholders across the ecosystem in order to ensure their use cases are
  addressed effectively and people have enough opportunity to migrate.

### Neutral

## References

- [\#6729]: Tendermint emits events over WebSocket faster than any clients can pull them if tx includes many events
- [\#7156]: Tracking: PubSub performance and UX improvements
- [RFC 003: Taxonomy of potential performance issues in Tendermint][rfc-003]
- [RFC 006: Event Subscription][rfc-006]
- [\#7471] Deterministic Events
- [ADR 075: RPC Event Subscription Interface][adr-075]

[\#6729]: https://github.com/tendermint/tendermint/issues/6729
[\#7156]: https://github.com/tendermint/tendermint/issues/7156
[PostgreSQL indexer]: https://github.com/tendermint/tendermint/blob/0f45086c5fd79ba47ab0270944258a27ccfc6cc3/state/indexer/sink/psql/psql.go
[\#7471]: https://github.com/tendermint/tendermint/issues/7471
[rfc-003]: ../rfc/rfc-003-performance-questions.md
[rfc-006]: ../rfc/rfc-006-event-subscription.md
[adr-075]: ./adr-075-rpc-subscription.md
[websocket-api]: https://docs.tendermint.com/v0.34/rpc/#/Websocket
[`/tx_search`]: https://docs.tendermint.com/v0.34/rpc/#/Info/tx_search
[`/block_search`]: https://docs.tendermint.com/v0.34/rpc/#/Info/block_search
[`/broadcast_tx_commit`]: https://docs.tendermint.com/v0.34/rpc/#/Tx/broadcast_tx_commit
[`/block_results`]: https://docs.tendermint.com/v0.34/rpc/#/Info/block_results
