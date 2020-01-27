# ADR 053: State Sync Prototype

## Changelog

* 2020-01-27: Initial draft (@erikgrinaker)

## Context

State sync will allow a new node to receive a snapshot of the application state without downloading blocks or going through consensus. This bootstraps the node much faster than the current fast sync, which replays all historical blocks.

The background discussions and recommended approach is detailed in [ADR-042](./adr-042-state-sync.md), and can be summarized as:

* The application periodically takes full state snapshots (i.e. eager snapshots).

* The application divides snapshots into a set of smaller chunks that can be separately verified against a chain Merkle root.

* Tendermint uses the light client to obtain a chain Merkle root for verification.

* Tendermint discovers and downloads snapshot chunks in parallel from multiple peers, and pass them to the application via ABCI to be applied and verified against the chain root.

* Historical blocks are not backfilled; state synced nodes will have a truncated block history.

This ADR describes the initial approach for a prototype implementation, and is subject to change as we receive feedback and gain more experience.

## Tendermint Proposal

This describes the snapshot/restore process seen from Tendermint's side. The guiding principle is to make the interface as small and general as possible, giving applications the maximum amount of flexibility.

### Snapshot Data Structure

Tendermint has minimal knowledge about _snapshots_ and snapshot _chunks_:

```proto
message Snapshot {
    uint64 height = 1;
    uint64 chunks = 2;
}

message SnapshotChunk {
    uint64 height = 1;
    uint64 chunk = 2;
    bytes data = 3;
}
```

A node can have multiple snapshots (taken at multiple heights), and each snapshot can have multiple chunks containing the actual data.

Data for chunk verification must be encoded in the `data` field, giving applications maximum flexibility in implementing this.

### ABCI Interface

Tendermint has minimal interfaces to list existing snapshots, fetch snapshot chunks, and apply snapshot chunks to a local ABCI application being state synced.

```proto
message RequestListSnapshots {}

message ResponseListSnapshots {
    repeated Snapshot = 1;
}

message RequestGetSnapshotChunk {
    uint64 height = 1;
    uint64 chunk = 2;
}

message ResponseGetSnapshotChunk {
    SnapshotChunk chunk = 1;
}

message RequestApplySnapshotChunk {
    SnapshotChunk chunk = 1;
    bytes chain_hash = 2;
    bool final = 3;
}

message ResponseApplySnapshotChunk {}
```

### Taking Snapshots

Tendermint is not aware of the snapshotting process at all, it is fully an application concern. This is non-trival, and applications must take care to provide the following guarantees:

* Snapshots must be taken periodically, not on-demand. This ensures faster restores, lower load, and removes a DoS vector.

* Snapshots must be deterministic, and identical across all nodes - typically by taking a snapshot at given height intervals.

* Snapshots must be consistent, i.e. not be affected by concurrent writes - typically by using a data store that supports versioning and/or snapshot isolation.

* Snapshots must be asynchronous, i.e. not halt block processing and state transitions.

* Snapshots must be split into chunks of reasonable size (on the order of megabytes), and each chunk must be verifiable against the chain Merkle root.

* Snapshots must be garbage collected periodically.

### Restoring Snapshots

When starting an empty node, there should be options for enabling fast sync and/or state sync. State sync requires a trusted header hash for the light client.

When starting an empty node with state sync and fast sync enabled, snapshots are restored as follows:

1. The node contacts the given seeds to discover peers.

2. The node checks that the local node is empty, i.e. that it has no state nor any applied packets.

2. The node contacts a full node, and verifies the trusted block header using the given hash.

3. The node requests available snapshots via `RequestListSnapshots`.

4. The node picks the latest valid snapshot. This must have height > trusted header and be within the unbonding period of the latest trustworthy height (as determined by the light client). If no valid snapshot is found, either try other peers, error out, or fall back to fast sync.

5. The node begins downloading chunks in parallel from multiple peers, via `RequestGetSnapshotChunk`.

6. The node passes chunks sequentially to the app, via `RequestApplySnapshotChunk`, along with the root chain hash at the height for verification. If verification or application fails, the application returns `ResponseException` and Tendermint tries refetching the chunk. The final chunk has `final = true`.

>>> Do we need error codes to be able to differentiate between invalid block, phony block, or internal error on apply?

7. Once all chunks have been applied, the node compares the app hash to the chain hash, and otherwise errors or discards the state and starts over.

8. The node switches to fast sync to catch up blocks that were committed while restoring the snapshot.

9. The node switches to normal consensus mode.

## Gaia Proposal

This describes the snapshot/restore process seen from Gaia's side.

## Status

Proposed

## Consequences

> This section describes the consequences, after applying the decision. All consequences should be summarized here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References

* [ADR-042](./adr-042-state-sync.md) and its references
