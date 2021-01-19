---
order: 5
---

# State Sync

## Channels

State sync has two distinct channels. The channel identifiers are listed below.

| Name            | Number |
|-----------------|--------|
| SnapshotChannel | 96     |
| ChunkChannel    | 97     |

## Message Types

### SnapshotRequest

When a new node begin state syncing, it will ask all peers it encounters if it has any
available snapshots:

| Name     | Type   | Description | Field Number |
|----------|--------|-------------|--------------|

### SnapShotResponse

The receiver will query the local ABCI application via `ListSnapshots`, and send a message
containing snapshot metadata (limited to 4 MB) for each of the 10 most recent snapshots: and stored at the application layer. When a peer is starting it will request snapshots.  

| Name     | Type   | Description                                               | Field Number |
|----------|--------|-----------------------------------------------------------|--------------|
| height   | uint64 | Height at which the snapshot was taken                    | 1            |
| format   | uint32 | Format of the snapshot.                                   | 2            |
| chunks   | uint32 | How many chunks make up the snapshot                      | 3            |
| hash     | bytes  | Arbitrary snapshot hash                                   | 4            |
| metadata | bytes  | Arbitrary application data. **May be non-deterministic.** | 5            |

### ChunkRequest

The node running state sync will offer these snapshots to the local ABCI application via
`OfferSnapshot` ABCI calls, and keep track of which peers contain which snapshots. Once a snapshot
is accepted, the state syncer will request snapshot chunks from appropriate peers:

| Name   | Type   | Description                                                 | Field Number |
|--------|--------|-------------------------------------------------------------|--------------|
| height | uint64 | Height at which the chunk was created                       | 1            |
| format | uint32 | Format chosen for the chunk.  **May be non-deterministic.** | 2            |
| index  | uint32 | Index of the chunk within the snapshot.                     | 3            |

### ChunkResponse

The receiver will load the requested chunk from its local application via `LoadSnapshotChunk`,
and respond with it (limited to 16 MB):

| Name    | Type   | Description                                                 | Field Number |
|---------|--------|-------------------------------------------------------------|--------------|
| height  | uint64 | Height at which the chunk was created                       | 1            |
| format  | uint32 | Format chosen for the chunk.  **May be non-deterministic.** | 2            |
| index   | uint32 | Index of the chunk within the snapshot.                     | 3            |
| hash    | bytes  | Arbitrary snapshot hash                                     | 4            |
| missing | bool   | Arbitrary application data. **May be non-deterministic.**   | 5            |

Here, `Missing` is used to signify that the chunk was not found on the peer, since an empty
chunk is a valid (although unlikely) response.

The returned chunk is given to the ABCI application via `ApplySnapshotChunk` until the snapshot
is restored. If a chunk response is not returned within some time, it will be re-requested,
possibly from a different peer.

The ABCI application is able to request peer bans and chunk refetching as part of the ABCI protocol.

If no state sync is in progress (i.e. during normal operation), any unsolicited response messages
are discarded.

### Message

Message is a [`oneof` protobuf type](https://developers.google.com/protocol-buffers/docs/proto#oneof). The `oneof` consists of four messages.

| Name               | Type                                  | Description                                  | Field Number |
|--------------------|---------------------------------------|----------------------------------------------|--------------|
| snapshots_request  | [SnapshotRequest](#snapshotrequest)   | Request a recent snapshot from a peer        | 1            |
| snapshots_response | [SnapshotResponse](#snapshotresponse) | Respond with the most recent snapshot stored | 2            |
| chunk_request      | [ChunkRequest](#chunkrequest)         | Request chunks of the snapshot.              | 3            |
| chunk_response     | [ChunkRequest](#chunkresponse)        | Response of chunks used to recreate state.   | 4            |
