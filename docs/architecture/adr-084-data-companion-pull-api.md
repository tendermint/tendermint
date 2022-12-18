# ADR 084: Data Companion Pull API

## Changelog

- 2022-12-18: First draft (@thanethomson)

## Status

Accepted | Rejected | Deprecated | Superseded by

## Context

Following from the discussion around the development of [ADR 082][adr-082], an
alternative model is proposed here for offloading certain data from
Tendermint-based nodes to a "data companion". This alternative model inverts the
control of the data offloading process, when compared to ADR 082, from the
Tendermint node to the data companion.

This particular model would

## Alternative Approaches

Other considered alternatives to this ADR are also outlined in
[ADR-082][adr-082].

## Decision

> This section records the decision that was made.

## Detailed Design

### Requirements

Similar requirements are proposed here as for [ADR-082][adr-082].

1. Only a single trusted companion service _must_ be supported in order to
   reduce the likelihood of overloading a node via the companion API. Use cases
   that require multiple companions will have to implement an intermediate/proxy
   companion that can scale independently of the node.

2. All or part of the following data _must_ be obtainable by the companion:
   1. Committed block data
   2. `FinalizeBlockResponse` data, but only for committed blocks

3. The companion _must_ be able to establish the earliest height for which the
   node has all of the requisite data.

4. The companion _must_ be able to establish, as close to real-time as possible,
   the height and ID of the block that was just committed by the network, so
   that it can request relevant heights' data from the node.

5. The API _must_ be (or be able to be) appropriately shielded from untrusted
   consumers and abuse. Critical control facets of the API (e.g. those that
   influence the node's pruning mechanisms) _must_ be implemented in such a way
   as to eliminate the possibility of accidentally exposing those endpoints to
   the public internet unprotected.

6. The node _must_ know, by way of signals from the companion, which heights'
   associated data are safe to automatically prune.

7. The API _must_ be opt-in. When off or not in use, it _should_ have no impact
   on system performance.

8. It _must_ not cause back-pressure into consensus.

8. It _must_ not cause unbounded memory growth.

10. It _must_ not cause unbounded disk storage growth.

11. It _must_ provide insight to operators (e.g. by way of logs/metrics) to
    assist in dealing with possible failure modes.

12. The solution _should_ be able to be backported to older versions of
    Tendermint (e.g. v0.34).

### Entity Relationships

The following model shows the proposed relationships between Tendermint, a
socket-based ABCI application, and the proposed data companion service.

```
     +----------+      +------------+      +----------------+
     | ABCI App | <--- | Tendermint | <--- | Data Companion |
     +----------+      +------------+      +----------------+
```

In this diagram, it is evident that Tendermint connects out to the ABCI
application, and the companion connects to the Tendermint node.

### gRPC API

At the time of this writing, it is proposed that Tendermint implement a full
gRPC interface ([\#9751]). As such, we have several options when it comes to
implementing the data companion pull API:

1. Extend the existing RPC API to simply provide the additional data
   companion-specific endpoints.
2. Implement a separate RPC API on a different port to the standard gRPC
   interface.

## Consequences

> This section describes the consequences, after applying the decision. All
> consequences should be summarized here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References

[adr-082]: https://github.com/tendermint/tendermint/pull/9437
[\#9751]: https://github.com/tendermint/tendermint/issues/9751
