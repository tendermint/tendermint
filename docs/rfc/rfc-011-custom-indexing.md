# RFC 011: Event Indexing Revisited

## Changelog

- 07-Feb-2022: Initial draft (@creachadair)

## Abstract

A Tendermint node allows ABCI events associated with block and transaction
processing to be "indexed" into persistent storage.  The original Tendermint
implementation provided a fixed, built-in [proprietary indexer][kv-index] for
such events.

In response to user requests to customize indexing, [ADR 065][adr065]
introduced an "event sink" interface that allows developers (at least in
theory) to plug in alternative index storage.

Although ADR-065 was a good first step toward customization, its implementation
model does not satisfy all the user requirements.  Moreover, this approach
leaves some existing technical issues with indexing unsolved.

This RFC documents these concerns, and discusses some potential approaches to
solving them.  This RFC does _not_ propose a specific technical decision. It is
meant to unify and focus some of the disparate discussions of the topic.


## Background

The [original indexer][kv-index] built in to Tendermint stored index data in an
embedded [`tm-db` database][tmdb] with a proprietary key layout.
In [ADR 065][adr065], we noted that this implementation has both performance
and scaling problems under load.  Moreover, the only practical way to query the
index data is via the [query filter language][query] used for event
subscription.  [Issue #1161][i1161] appears to be a motivational context for that ADR.

To mitigate both of these concerns, we introduced the [`EventSink`][esink]
interface, combining the original transaction and block indexer interfaces
along with some service plumbing.  Using this interface, a developer can plug
in an indexer that uses a more efficient storage engine, and provides a more
expressive query language.  As a proof-of-concept, we built a [PostgreSQL event
sink][psql] that exports data to a [PostgreSQL database][postgres].

Although this approach addressed some of the immediate concerns, there are
several issues for custom indexing that have not been fully addressed. Here we
will discuss them in more detail.

For further context, including links to user reports and related work, see also
the [Pluggable custom event indexing tracking issue][i7135] issue.

### Issue 1: Tight Coupling

The `EventSink` interface supports multiple implementations, but plugging in
implementations still requires tight integration with the node. In particular:

- Any custom indexer must either be written in Go and compiled in to the
  Tendermint binary, or the developer must write a Go shim to communicate with
  the implementation and build that into the Tendermint binary.

- This means to support a custom indexer, it either has to be integrated into
  the Tendermint core repository, or every installation that uses that indexer
  must fetch or build a patched version of Tendermint.

The problem with integrating indexers into Tendermint Core is that every user
of Tendermint Core takes a dependency on all supported indexers, including
those they never use. Even if the unused code is disabled with build tags,
users have to remember to do this or potentially be exposed to security issues
that may arise in any of the custom indexers. This is a risk for Tendermint,
which is a trust-critical component of all applications built on it.

The problem with _not_ integrating indexers into Tendermint Core is that any
developer who wants to use a particular indexer must now fetch or build a
patched version of the core code that includes the custom indexer. Besides
being inconvenient, this makes it harder for users to upgrade their node, since
they need to either re-apply their patches directly or wait for an intermediary
to do it for them.

Even for developers who have written their applications in Go and link with the
consensus node directly (e.g., using the [Cosmos SDK][sdk]), these issues add a
potentially significant complication to the build process.

### Issue 2: Legacy Compatibility

The `EventSink` interface retains several limitations of the original
proprietary indexer. These include:

- The indexer has no control over which events are reported. Only the exact
  block and transaction events that were reported to the original indexer are
  reported to a custom indexer.

- The interface requires the implementation to define methods for the legacy
  search and query API. This requirement comes from the integation with the
  [event subscription RPC API][event-rpc], but actually supporting these
  methods is not trivial.

At present, only the original KV indexer implements the query methods. Even the
proof-of-concept PostgreSQL implementation simply reports errors for all calls
to these methods.

Even for a plugin written in Go, implementing these methods "correctly" would
require parsing and translating the custom query language over whatever storage
platform the indexer uses.

For a plugin _not_ written in Go, even beyond the cost of integration the
developer would have to re-implement the entire query language.

### Issue 3: Indexing Delays Consensus

Within the node, indexing hooks in to the same internal pubsub dispatcher that
is used to export events to the [event subscription RPC API][event-rpc]. In
contrast with RPC subscribers, however, indexing is a "privileged" subscriber:
If an RPC subscriber is "too slow", the node may terminate the subscription and
disconnect the client. That means that RPC subscribers may lose (miss) events.
The indexer, however, is "unbuffered", and the publisher will never drop or
disconnect from it. If the indexer is slow, the publisher will block until it
returns, to ensure that no events are lost.

In practice, this means that the performance of the indexer has a direct effect
on the performance of the consensus node: If the indexer is slow or stalls, it
will slow or halt the progress of consensus. Users have already reported this
problem even with the built-in indexer (see, for example, [#7247][i7247]).
Extending this concern to arbitrary user-defined custom indexers gives that
risk a much larger surface area.


## Discussion

Although there is no unique "best" solution to the issues described above,
there are some specific principles that a solution should include:

1. **A custom indexer should not require integration into Tendermint core.** A
   developer or node operator can create, build, deploy, and use a custom
   indexer with a stock build of the Tendermint consensus node.

2. **Custom indexers cannot stall consensus.** An indexer that is slow or
   stalls cannot slow down or prevent core consensus from making progress.

   The plugin interface must give node operators control over the tolerances
   for acceptable indexer performance, and the means to detect when indexers
   are falling outside those tolerances, but indexer failures should "fail
   safe" with respect to consensus (even if that means the indexer may miss
   some data, in sufficiently-extreme circumstances).

3. **Custom indexers control which events they index.** A custom indexer is not
   limited to only the current transaction and block events, but can observe
   any event published by the node.

4. **Custom indexing is forward-compatible.** Adding new event types or event
   data to the consensus node should not require existing custom indexers to be
   rebuilt or modified, unless they want to take advantage of the new data.

5. **Indexers are responsible for answering queries.** An indexer plugin is not
   required to support the legacy query filter language, nor to be compatible
   with the legacy RPC endpoints for accessing them.  Any APIs for clients to
   query a custom index are the responsibility of the indexer, not the node.

### Informal Design Intent

The design principles described above implicate several components of the
Tendermint node, beyond just the indexer. In the context of [ADR 075][adr075],
we are re-working the RPC event subscription API to improve some of the UX
issues discussed above for RPC clients. It is our expectation that a solution
for pluggable custom indexing will take advantage of some of the same work.

On that basis, the design approach I am considering for custom indexing looks
something like this (subject to refinement):

1. A custom indexer runs as a separate process from the node.

2. The indexer subscribes to events via the (ADR 075) event subscription API.

   This means indexers would receive event payloads as JSON rather than
   protobuf, but since we already have to support JSON encoding for the RPC
   interface anyway, that should not increase complexity for the node.

3. The existing PostgreSQL indexer gets reworked to have this form, and no
   longer built as part of the Tendermint core binary.

   We can retain the code in the core repository as a proof-of-concept, or
   perhaps create a separate repository with contributed indexers and move it
   there.

4. (Possibly) Deprecate and remove the legacy KV indexer, or disable it by
   default.  If we decide to remove it, we can also remove the legacy RPC
   endpoints for querying the KV indexer.


## References

- [ADR 065: Custom Event Indexing][adr065]
- [ADR 075: RPC Event Subscription Interface][adr075]
- [Cosmos SDK][sdk]
- [Event subscription RPC][event-rpc]
- [KV transaction indexer][kv-index]
- [Pluggable custom event indexing][i7135] (#7135)
- [PostgreSQL event sink][psql]
   - [PostgreSQL database][postgres]
- [Query filter language][query]
- [Stream events to postgres for indexing][i1161] (#1161)
- [Unbuffered event subscription slow down the consensus][i7247] (#7247)
- [`EventSink` interface][esink]
- [`tm-db` library][tmdb]

[adr065]: https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-065-custom-event-indexing.md
[adr075]: https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-075-rpc-subscription.md
[esink]: https://pkg.go.dev/github.com/tendermint/tendermint/internal/state/indexer#EventSink
[event-rpc]: https://docs.tendermint.com/master/rpc/#/Websocket/subscribe
[i1161]: https://github.com/tendermint/tendermint/issues/1161
[i7135]: https://github.com/tendermint/tendermint/issues/7135
[i7247]: https://github.com/tendermint/tendermint/issues/7247
[kv-index]: https://github.com/tendermint/tendermint/blob/master/internal/state/indexer/tx/kv
[postgres]: https://postgresql.org/
[psql]: https://github.com/tendermint/tendermint/blob/master/internal/state/indexer/sink/psql
[psql]: https://github.com/tendermint/tendermint/blob/master/internal/state/indexer/sink/psql
[query]: https://pkg.go.dev/github.com/tendermint/tendermint/internal/pubsub/query/syntax
[sdk]: https://github.com/cosmos/cosmos-sdk
[tmdb]: https://pkg.go.dev/github.com/tendermint/tm-db#DB
