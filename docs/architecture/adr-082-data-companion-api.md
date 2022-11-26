# ADR 082: Data Companion API

## Changelog

- 2022-11-26: Clarify user stories and alternatives, allow for selective
  publishing of data via the companion API, buffer on disk instead of in memory
  (@thanethomson)
- 2022-09-10: First draft (@thanethomson)

## Status

Accepted | Rejected | Deprecated | Superseded by

## Context

This ADR proposes to effectively offload some data storage responsibilities and
functionality to a **single trusted external companion service** such that:

1. Integrators can expose Tendermint data in whatever format/protocol they want
   (e.g. JSON-RPC or gRPC).
2. Integrators can index data in whatever way suits them best.
3. All users eventually benefit from faster changes to Tendermint because the
   core team has less, and less complex, code to maintain (implementing this ADR
   would add more code in the short-term, but would pave the way for significant
   reductions in complexity in future).

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

1. Block data (i.e. `Block` data structures that have been committed by the
   consensus engine).
2. The results of block execution (e.g. data from `FinalizeBlockResponse`). This
   data is not accessible from the P2P layer, and currently provides valuable
   information for Tendermint integrators (whether events should be handled at
   all by Tendermint is a different problem).

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
   database would be the best option here, as that would most likely be
   influenced by integrators' use cases.

   Beyond this, if we exclusively support one database but integrators needed
   another for their use case, they would be forced to perform some kind of
   continuous ETL operation to transfer the data from the one database to the
   other, potentially doubling their storage capacity requirements and therefore
   their storage-related costs. This would also increase pruning complexity.

## Decision

> TODO(thane)

## Detailed Design

### Use Cases

1. Integrators could provide standalone RPC services that would be able to scale
   independently of the Tendermint node. They would also be able to present the
   data in whatever format and by whichever protocol they want.
2. Block explorers could make use of this data to provide real-time information
   about the blockchain.
3. IBC relayer nodes could use this data to filter and store only the data they
   need to facilitate relaying without putting additional pressure on the
   consensus engine (until such time that a decision is made on whether to
   continue providing event data from Tendermint).

### Requirements

1. All of the following data must be published to a single external consumer:
   1. Committed block data
   2. `FinalizeBlockResponse` data, but only for committed blocks

2. It must be opt-in. In other words, it is off by default and turned on by way
   of a configuration parameter. When off, it must have negligible (ideally no)
   impact on system performance.

3. It must not cause back-pressure into consensus.

4. It must not cause unbounded memory growth.

5. It must not cause unbounded disk storage growth.

6. Data must be published reliably. If data cannot be published, the node must
   rather crash in such a way that bringing the node up again will cause it to
   attempt to republish the unpublished data.

7. The interface must support extension by allowing more kinds of data to be
   exposed via this API (bearing in mind that every extension has the potential
   to affect system stability in the case of an unreliable data companion
   service).

8. Integrators must have some control over which broad types of data are
   published.

### Entity Relationships

The following simple diagram shows the proposed relationships between
Tendermint, a socket-based ABCI application, and the proposed data companion
service.

```
     +----------+      +------------+      +----------------+
     | ABCI App | <--- | Tendermint | ---> | Data Companion |
     +----------+      +------------+      +----------------+

```

As can be seen in this diagram, Tendermint connects out to both the ABCI
application and data companion service based on the Tendermint node's
configuration.

The fact that Tendermint connects out to the companion service instead of the
other way around provides a natural constraint on the number of consumers of the
API. This also implies that the data companion is a trusted external service,
just like an ABCI application, as it would need to be configured within the
Tendermint configuration file.

### gRPC API

The following gRPC API is proposed:

```protobuf
import "tendermint/abci/types.proto";
import "tendermint/types/block.proto";

// DataCompanionService allows Tendermint to publish certain data generated by
// the consensus engine to a single external consumer with specific reliability
// guarantees.
//
// Note that implementers of this service must take into account the possibility
// that Tendermint may re-send data that was previously sent. Therefore
// the service should simply ignore previously seen data instead of responding
// with errors to ensure correct functioning of the node, as responding with an
// error is taken as a signal to Tendermint that it must crash and await
// operator intervention.
service DataCompanionService {
    // BlockCommitted is called after a block has been committed. This method is
    // also called on Tendermint startup to ensure that the service received the
    // last committed block, in case there was previously a transport failure.
    //
    // If an error is returned, Tendermint will crash.
    rpc BlockCommitted(BlockCommittedRequest) returns (BlockCommittedResponse) {}
}

// BlockCommittedRequest contains at least the data for the block that was just
// committed. If enabled, it also contains the ABCI FinalizeBlock response data
// related to the block in this request.
message BlockCommittedRequest {
    // The block data for the block that was just committed.
    tendermint.types.Block                         block = 1;
    // The FinalizeBlockResponse related to the block in this request. This
    // field is optional, depending on the Tendermint node's configuration.
    optional tendermint.abci.FinalizeBlockResponse finalize_block_response = 2;
}

// BlockCommittedResponse is either empty upon succes, or returns one or more
// errors. Note that returning any errors here will cause Tendermint to crash.
message BlockCommittedResponse {
    // Zero or more errors occurred during the companion's processing of the
    // request.
    repeated Error errors = 1;
}

// Error can be one of several different types of errors.
message Error {
    oneof error {
        UnexpectedBlockError unexpected_block_err = 1;
        UnexpectedFieldError unexpected_field_err = 2;
        ExpectedFieldError   expected_field_err   = 3;
    }
}

// UnexpectedBlockError is returned by the server when Tendermint sent it a
// block that is ahead of the block expected by the server.
message UnexpectedBlockError {
    // The height of the block expected by the server.
    int64 expected_height = 1;
}

// UnexpectedFieldError is returned by the server when Tendermint sent it a
// message containing an unexpected field. For instance, if the companion
// expects only the `block` field but not `finalize_block_response`, but somehow
// the companion receives a value for `finalize_block_response`, it should
// return this error because it indicates that either Tendermint or the data
// companion are most likely incorrectly configured.
message UnexpectedFieldError {
    // The name of the field whose value was expected to be empty, but was not.
    string field_name = 1;
}

// ExpectedFieldError is returned by the server when Tendermint sent it a
// message containing an empty field value for a field it was expecting to be
// populated. If this occurs, it indicates that either Tendermint or the data
// companion are most likely incorrectly configured.
message ExpectedFieldError {
    // The name of the field whose value was expected to be populated, but was
    // empty.
    string field_name = 1;
}
```

### Request Buffering

In order to ensure reliable delivery, while also catering for intermittent
faults, we should facilitate buffering of data destined for the companion
service. This data would need to be stored on disk to ensure that Tendermint
could attempt to resubmit all unsubmitted data upon Tendermint startup.

At present, this data is already stored on disk, and so practically we would
need to implement some form of background pruning mechanism to remove the data
we know has been shared with the companion service.

In order to not allow disk storage to grow unbounded, _and_ ensure that we do
not lose any critical data, we need to impose some kind of limit on the number
of requests we will buffer before crashing the Tendermint node. In case there is
manual intervention needed to restore the companion service, Tendermint should
attempt to flush this buffer upon startup before continuing normal operations.

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

# Use the experimental gzip compression call option when submitting data to the
# server. See https://pkg.go.dev/google.golang.org/grpc#UseCompressor
experimental_use_gzip = false

# Controls the BlockCommitted gRPC call.
[data_companion.block_committed]
# Enable the BlockCommitted gRPC call. Only relevant if the data companion is
# enabled.
enabled = true
# Additional fields to publish in each BlockCommittedRequest sent to the
# companion. Available options:
# - "finalize_block_response": Also publish the FinalizeBlock response related
#                              to the block in the BlockCommittedRequest.
additionally_publish = ["finalize_block_response"]
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
- `data_companion_send_bytes` - A counter indicating the number of bytes sent to
  the data companion since node startup.
- `data_companion_buffer_size` - A gauge indicating the number of requests
  currently being buffered on disk on behalf of the companion service for each
  kind of request.
- `data_companion_buffer_size_bytes` - A gauge indicating how much data is being
  buffered by Tendermint on disk, in bytes, on behalf of the companion service,
  for each kind of request.

## Implications

1. We will be able to mark the following RPC APIs as deprecated and schedule
   them for removal in a future release:
   1. The [WebSocket subscription API][websocket-api]
   2. [`/tx_search`]
   3. [`/block_search`]
   4. [`/block_results`]

   We may eventually be able to remove the [`/broadcast_tx_commit`] endpoint,
   but this is often useful during debugging and development, so we may want to
   keep it around.

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
