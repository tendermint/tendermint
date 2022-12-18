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
   companion-specific endpoints. In order to meet the
   [requirements](#requirements), however, some of the endpoints will have to be
   protected by default. This is simpler for clients to interact with though,
   because they only need to interact with a single endpoint for all of their
   needs.
2. Implement a separate RPC API on a different port to the standard gRPC
   interface. This allows for a clearer distinction between the standard and
   data companion-specific gRPC interfaces, but complicates the server and
   client interaction models.

Due to the poorer operator experience in option 2, it would be preferable to
implement option 1, but have certain endpoints be
[access-controlled](#access-control) by default.

With this in mind, the following gRPC API is proposed, where the Tendermint node
will implement these services.

```protobuf
import "tendermint/abci/types.proto";
import "tendermint/types/types.proto";
import "tendermint/types/block.proto";

// BlockService provides information about blocks.
service BlockService {
    // GetLatestBlockID returns a stream of the latest block IDs as they are
    // committed by the network.
    rpc GetLatestBlockID(GetLatestBlockIDRequest) returns (stream GetLatestBlockIDResponse) {}

    // GetBlock attempts to retrieve the block at a particular height.
    rpc GetBlock(GetBlockRequest) returns (GetBlockResponse) {}

    // GetBlockResults attempts to retrieve the results of block execution for a
    // particular height.
    rpc GetBlockResults(GetBlockResultsRequest) returns (GetBlockResultsResponse) {}
}

// DataCompanionService provides privileged access to specialized pruning
// functionality on the Tendermint node to help optimize node storage.
service DataCompanionService {
    // SetRetainHeight notifies the node of the minimum height whose data must
    // be retained by the node. This data includes block data and block
    // execution results.
    //
    // The lower of this retain height and that set by the application in its
    // Commit response will be used by the node to determine which heights' data
    // can be pruned.
    rpc SetRetainHeight(SetRetainHeightRequest) returns (SetRetainHeightResponse) {}

    // GetRetainHeight returns the retain height set by the companion and that
    // set by the application. This can give the companion an indication as to
    // which heights' data are currently available.
    rpc GetRetainHeight(GetRetainHeightRequest) returns (GetRetainHeightResponse) {}
}

message GetLatestBlockIDRequest {}

// GetLatestBlockIDResponse is a lightweight reference to the latest committed
// block.
message GetLatestBlockIDResponse {
    // The height of the latest committed block.
    int64 height = 1;
    // The ID of the latest committed block.
    tendermint.types.BlockID block_id = 2;
}

message GetBlockRequest {
    // The height of the block to get.
    int64 height = 1;
}

message GetBlockResponse {
    // Block data for the requested height.
    tendermint.types.Block block = 1;
}

message GetBlockResultsRequest {
    // The height of the block results to get.
    int64 height = 1;
}

message GetBlockResultsResponse {
    // All events produced by the ABCI BeginBlock call for the block.
    repeated tendermint.abci.Event begin_block_events = 1;

    // All transaction results produced by block execution.
    repeated tendermint.abci.ExecTxResult tx_results = 2;

    // Validator updates during block execution.
    repeated tendermint.abci.ValidatorUpdate validator_updates = 3;

    // Consensus parameter updates during block execution.
    tendermint.types.ConsensusParams consensus_param_updates = 4;

    // All events produced by the ABCI EndBlock call.
    // NB: This should be called finalize_block_events when ABCI 2.0 lands.
    repeated tendermint.abci.Event end_block_events = 5;
}

message SetRetainHeightRequest {
    int64 height = 1;
}

message SetRetainHeightResponse {}

message GetRetainHeightRequest {}

message GetRetainHeightResponse {
    // The retain height as set by the data companion.
    int64 data_companion_retain_height = 1;
    // The retain height as set by the ABCI application.
    int64 app_retain_height = 2;
    // The height whose data is currently retained, which is influenced by the
    // data companion and ABCI application retain heights, but the node may not
    // yet have executed its pruning operation.
    int64 actual_retain_height = 3;
}
```

### Access Control

As covered in the [gRPC API section](#grpc-api), it would be preferable to
implement some form of access control for sensitive, data companion-specific
APIs. At least **basic HTTP authentication** should be implemented for these
endpoints, where credentials should be obtained (in order of precedence):

1. From an `.htpasswd` file, using the same format as [Apache `.htpasswd`
   files][htpasswd], whose location is set in the Tendermint configuration file.
2. Randomly generated and written to the logs.

### Configuration

The following configuration file update is proposed to support the data
companion API.

## Consequences

> This section describes the consequences, after applying the decision. All
> consequences should be summarized here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References

[adr-082]: https://github.com/tendermint/tendermint/pull/9437
[\#9751]: https://github.com/tendermint/tendermint/issues/9751
[htpasswd]: https://httpd.apache.org/docs/current/programs/htpasswd.html
