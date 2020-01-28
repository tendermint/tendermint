# ADR 053: State Sync Prototype

This ADR describes an initial state sync prototype, and is subject to change as we gain feedback and experience.

## Changelog

* 2020-01-27: Initial draft (@erikgrinaker)

## Context

State sync will allow a new node to receive a snapshot of the application state without downloading blocks or going through consensus. This bootstraps the node significantly faster than the current fast sync system, which replays all historical blocks.

Background discussions and justifications are detailed in [ADR-042](./adr-042-state-sync.md), which can be summarized as:

* The application periodically takes full state snapshots (i.e. eager snapshots).

* The application splits snapshots into smaller chunks that can be individually verified against a chain Merkle root.

* Tendermint uses the light client to obtain a trusted chain Merkle root for verification.

* Tendermint discovers and downloads snapshot chunks in parallel from multiple peers, and passes them to the application via ABCI to be applied and verified against the chain root.

* Historical blocks are not backfilled, so state synced nodes will have a truncated block history.

## Tendermint Proposal

This describes the snapshot/restore process seen from Tendermint. The interface is kept as small and general as possible to give applications maximum flexibility.

### Snapshot Data Structure

```proto
message Snapshot {
    uint64 height = 1;   // The height at which the snapshot was taken
    uint64 format = 2;   // The application-specific snapshot format version
    uint64 chunks = 3;   // The number of chunks in the snapshot
    bytes metadata = 4;  // Arbitrary application metadata
}

message SnapshotChunk {
    uint64 height = 1;   // The height of the corresponding snapshot
    uint64 format = 2;   // The snapshot format version
    uint64 chunk = 3;    // The chunk index (zero-based)
    bytes data = 4;      // Serialized application state in an arbitrary format
}
```

A node can have multiple snapshots taken at various heights, in different application-specific format versions. Each snapshot consists of multiple chunks containing the actual state data.

Chunk verification data must be encoded along with the state data in the `data` field.

### ABCI Interface

```proto
// Lists available snapshots
message RequestListSnapshots {}

message ResponseListSnapshots {
    repeated Snapshot snapshots = 1;
}

// Offers a snapshot to the application
message RequestOfferSnapshot {
    Snapshot snapshot = 1;
}

message ResponseOfferSnapshot {
    bool accepted = 1;
}

// Fetches a snapshot chunk
message RequestGetSnapshotChunk {
    uint64 height = 1;
    uint64 format = 2;
    uint64 chunk = 3;
}

message ResponseGetSnapshotChunk {
    SnapshotChunk chunk = 1;
}

// Applies a snapshot chunk
message RequestApplySnapshotChunk {
    SnapshotChunk chunk = 1;
    bytes chain_hash = 2;
    bool final = 3;
}

message ResponseApplySnapshotChunk {}
```

### Taking Snapshots

Tendermint is not aware of the snapshotting process at all, it is entirely an application concern. The following guarantees must be provided:

* **Periodic:** snapshots must be taken periodically, not on-demand, for faster restores, lower load, and less DoS risk.

* **Deterministic:** snapshots must be deterministic, and identical across all nodes - typically by taking a snapshot at given height intervals.

* **Consistent:** snapshots must be consistent, i.e. not affected by concurrent writes - typically by using a data store that supports versioning and/or snapshot isolation.

* **Asynchronous:** snapshots must be asynchronous, i.e. not halt block processing and state transitions.

* **Chunked:** snapshots must be split into chunks of reasonable size (on the order of megabytes), and each chunk must be verifiable against the chain Merkle root.

* **Garbage collected:** snapshots must be garbage collected periodically.

### Restoring Snapshots

Nodes should have options for enabling state sync and/or fast sync, and be provided a trusted header hash for the light client.

When starting an empty node with state sync and fast sync enabled, snapshots are restored as follows:

1. The node checks that it is empty, i.e. that it has no state nor blocks.

2. The node contacts the given seeds to discover peers.

3. The node contacts a set of full nodes, and verifies the trusted block header using the given hash via the light client.

4. The node requests available snapshots via `RequestListSnapshots`.

5. The node iterates over all snapshots in reverse order by height and format until it finds one that satisfies the following conditions:

    * The snapshot height's block is considered trustworthy by the light client (i.e. snapshot height is greater than trusted header and within unbonding period of the latest trustworthy block).

    * The application accepts the `RequestOfferSnapshot` call.

6. The node downloads chunks in parallel from multiple peers via `RequestGetSnapshotChunk`.

7. The node passes chunks sequentially to the app via `RequestApplySnapshotChunk`, along with the root chain hash at the snapshot height for verification. If the chunk cannot be verified or applied, the application returns `ResponseException` and Tendermint tries refetching the chunk. The final chunk has `final: true`.

8. Once all chunks have been applied, the node compares the app hash to the chain hash, and if they do not match it either errors or discards the state and starts over.

9. The node switches to fast sync to catch up blocks that were committed while restoring the snapshot.

10. The node switches to normal consensus mode.

## Gaia Proposal

This describes the snapshot process seen from Gaia's side, using format version `1`. The serialization format is unspecified, but likely to be compressed Amino or Protobuf.

### Snapshot Metadata

In the initial version there is no snapshot metadata, so it is set to an empty byte buffer.

Once all chunks have been successfully built, snapshot metadata should be serialized and stored in the file system as e.g. `snapshots/<height>/<format>/metadata`, and served via `RequestListSnapshots`.

### Snapshot Chunk Format

The Gaia data structure consists of a set of named IAVL trees. A root hash is constructed by taking the root hashes of each of the IAVL trees, then constructing a Merkle tree of the sorted name/hash map.

IAVL trees are versioned, but a snapshot only contains the version relevant for the snapshot height. All historical versions are ignored.

IAVL trees are insertion-order dependent, so key/value pairs must be stored in an appropriate insertion order to produce the same tree branching structure and thus the same Merkle hashes.

A chunk corresponds to a subtree key range within an IAVL tree, in insertion order, along with the Merkle proof from the root of the subtree all the way up to the root of the multistore (including the Merkle proof for the surrounding multistore.

```go
struct SnapshotChunk {
    Store   string            // Name (key) of IAVL store in outer MultiStore
    Keys    [][]byte          // Snapshotted keys in insertion order
    Values  [][]byte          // Snapshotted values corresponding to Keys
    Proof   []merkle.ProofOp  // Merkle proof from subtree root to MultiStore root
}
```

This chunk structure is believed to be sufficient to reconstruct an identical IAVL tree by applying separate chunks, but this may require the chunks to be ordered in a certain way; further research is needed.

We do not use IAVL RangeProofs, since these include redundant data such as proofs for intermediate and leaf nodes that can be derived from the above data.

Chunks should be built greedily by collecting key/value pairs constituting a complete subtree up to some size limit (e.g. 16 MB), then serialized and stored in the file system as `snapshots/<height>/<format>/<chunk>/data` and served via `RequestGetSnapshotChunk`.

### Snapshot Scheduling

Snapshots should be taken at some configurable height interval, e.g. every 1000 blocks. All nodes should preferably have the same snapshot schedule, such that all nodes can serve chunks for a given snapshot.

Taking consistent snapshots of IAVL trees is greatly simplified by them being versioned: simply snapshot the version that corresponds to the snapshot height, while concurrent writes create new versions. IAVL pruning must not prune a version that is being snapshotted.

Snapshots must also be garbage collected after some configurable time, e.g. by keeping the latest `n` snapshots.

## Open Questions

* Is it possible to reconstruct an identical IAVL tree given separate subtrees in an appropriate order, or is more data needed about the branch structure?

* Should `ResponseOfferSnapshot` and `ResponseApplySnapshotChunk` return specific codes for e.g. invalid heights, unknown formats, and so on?

* Is it OK for state-synced nodes to not have historical blocks nor historical IAVL versions?

* Do we need to reject chunk sizes beyond a configurable limit, or will this be handled implicitly by e.g. gRPC request size limits?

## Status

Proposed

## References

* [ADR-042](./adr-042-state-sync.md) and its references
