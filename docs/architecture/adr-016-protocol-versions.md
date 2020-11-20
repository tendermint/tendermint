# ADR 016: Protocol Versions

## Changelog

- 11-11-2020: Address both RPC and ABCI versioning
- 18-09-2018: Updates after working a bit on implementation
  - ABCI Handshake needs to happen independently of starting the app conns so we
    can see the result
  - Add question about ABCI protocol version
- 16-08-2018: Updates after discussion with SDK team
  - Remove signalling for next version from Header/ABCI
- 03-08-2018: Updates from discussion with Jae:
  - ProtocolVersion contains Block/AppVersion, not Current/Next
  - signal upgrades to Tendermint using EndBlock fields
  - don't restrict peer compatibility by version to simplify syncing old nodes
- 28-07-2018: Updates from review
  - split into two ADRs - one for protocol, one for chains
  - include signalling for upgrades in header
- 16-07-2018: Initial draft - was originally joint ADR for protocol and chain
  versions

## Context

This ADR focuses on software-agnostic protocol versions.

The Software Version is covered by SemVer and described elsewhere.
It is not relevant to the protocol description, suffice to say that if any
protocol version changes, the software version changes, but not necessarily
vice versa.

Software version should be included in NodeInfo for convenience/diagnostics.

We need to version components of the blockchain that may be independently
upgraded. We need to do it in a way that is scalable and maintainable - we can't
just litter the code with conditionals. We need a versioning system that
supports a range of stakeholders throughout the lifespan of a blockchain.

We also need to consider the lifespan of the application itself and be
accommodating to how it upgrades.

## Proposal

Ideally, each component of the software is independently versioned in a modular
way and its easy to mix and match and upgrade.

We can consider the complete version of the protocol to contain the following
sub-versions: **BlockVersion**, **P2PVersion**, **AppVersion**.
These versions reflect the major sub-components of the software that are likely
to evolve together, at different rates, and in different ways.

### What does each protocol version correspond to and how are they allowed to evolve

Each of BlockVersion, AppVersion, and P2PVersion is a monotonically increasing
uint64. Incrementing the version number indicates a change that is incompatible
with the previous version.

#### BlockVersion

- This consists of all Tendermint data-structures (headers, votes, commits, txs,
  responses, etc.). A complete list of data structures can be found in the
  [spec](https://github.com/tendermint/spec/blob/master/spec/core/data_structures.md)

#### P2PVersion

- All p2p and reactor messaging (messages, detectable behavior)
- It should be easy to determine the version of a peer from its first serialized
  message/s
- New versions must be compatible with at least one old version to allow gradual
  upgrades
- We need the peer/reactor protocols to take the versions of peers into account
  when sending messages so that we don't send messages that the peer won't
  understand or won't expect.

#### AppVersion

- The ABCI state machine itself.
- Tendermint needs to know the version of the application to ensure that AppHash
  and Results are calculated in the same manner across the entire network.

In addition, the block version is the parent of two further sub versions:
**RPCVersion** and **ABCIVersion**.These are denoted using semantic versioning. 
Changing the block protocol will require either a new major or minor release 
for each of these versions as the data structures they serve have changed, 
however major and minor releases wouldn't necessarily mean a block protocol change.

#### RPCVersion

This covers all RPC endpoints and their respective requests and responses.
In the case of an introduction of a new interface with external users (i.e gRPC),
this would also fall under the RPCVersion. Minor changes could be adding new
endpoints or adding fields so long as these fields would not affect verification
(in other words, doesn't affect a hash).

#### ABCIVersion

Refers to the entire ABCI Library and predominantly the application interface.
Similarly with RPCVersion, minor changes could be adding fields or calls
(so long as they are optional and not connected with the replication logic).
Because the application isn't concerned with verification logic or assessing hashes,
a block protocol version change does not necessarily imply a major version change.
A minor version change could suffice.

### Where are versions located and how are they updated

To use these versions, we need to update the block Header, the p2p NodeInfo, the
ABCI and add an RPC endpoint.

#### Header

Block Header should include a `Version` struct as its first field like:

```
type Version struct {
    Block uint64
    App uint64
}
```

Here, `Version.Block` defines the rules for the current block, while
`Version.App` defines the app version that processed the last block and computed
the `AppHash` in the current block. Together they provide a complete description
of the consensus-critical protocol.

Since we have settled on a proto3 header, the ability to read the BlockVersion
out of the serialized header is unanimous.

Using a Version struct gives us more flexibility to add fields without breaking
the header.

#### NodeInfo

NodeInfo should include a Version struct as its first field like:

```golang
type Version struct {
    P2P uint64
    Block uint64
    App uint64

    Other []string
}
```

Note this effectively makes `Version.P2P` the first field in the NodeInfo, so it
should be easy to read this out of the serialized header if need be to
facilitate an upgrade.

The `Version.Other` here should include additional information like the name of
the software client and it's SemVer version - this is for convenience only.
Eg. `tendermint-core/v0.22.8`. It's a `[]string` so it can include information
about the version of Tendermint, of the app, of Tendermint libraries, etc.

Note NodeInfo is only exchanged after the authenticated encryption handshake to
ensure that it's private. Doing any version exchange before encrypting could be
considered information leakage, though I'm not sure how much that matters
compared to being able to upgrade the protocol.

#### ABCI

Since the ABCI is responsible for keeping Tendermint and the App in sync, we
need to communicate version information through it.

On startup, we use Info to perform a basic handshake. It should include all the
version information so the node can check for compatibility. This would also
include tha ABCI version so applications using a socket connection can ensure
they are using the correct version.

##### Info

RequestInfo should add support for protocol versions like:

```golang
message RequestInfo {
  string abci_version
  string version
  uint64 block_version
  uint64 p2p_version
}
```

Similarly, ResponseInfo should return the versions:

```golang
message ResponseInfo {
  string data

  string version
  uint64 app_version

  int64 last_block_height
  bytes last_block_app_hash
}
```

The existing `version` field should be called `software_version` but we will leave it for now to reduce the number of breaking changes.

We also need to be able to update versions throughout the life of a blockchain. The natural place to do this is EndBlock.

##### EndBlock

Updating the version could be done either with new fields or by using the
existing `tags`. Since we're trying to communicate information that will be
included in Tendermint block Headers, it should be native to the ABCI, and not
something embedded through some scheme in the tags. Thus, version updates should
be communicated through EndBlock.

EndBlock already contains `ConsensusParams`. We can add version information to
the ConsensusParams as well:

```golang
message ConsensusParams {

  BlockSize block_size
  EvidenceParams evidence_params
  VersionParams version
}

message VersionParams {
    uint64 block_version
    uint64 app_version
}
```

For now, the `block_version` will be ignored, as we do not allow block version
to be updated live. If the `app_version` is set, it signals that the app's
protocol version has changed, and the new `app_version` will be included in the
`Block.Header.Version.App` for the next block.

#### RPC

Clients can communicate the RPC version that they are able to parse in the request.
This can be either through the base URL path or in the header.
If the requested version is incompatible with the node's RPCVersion (i.e. different major versions)
then it will return a version error. If the client can support multiple RPC versions
then it can use the `/version` endpoint to ascertain the exact RPCVersion of the node.

### Peer Compatibility

Restricting peer compatibility based on version is complicated by the need to
help old peers, possibly on older versions, sync the blockchain.

We might be tempted to say that we only connect to peers with the same
AppVersion and BlockVersion (since these define the consensus critical
computations), and a select list of P2PVersions (ie. those compatible with
ours), but then we'd need to make accommodations for connecting to peers with the
right Block/AppVersion for the height they're on.

For now, we will connect to peers with any version and restrict compatibility
solely based on the ChainID. We leave more restrictive rules on peer
compatibility to a future proposal.

### Future Changes

It may be valuable to support an `/unsafe_stop?height=_` endpoint to tell Tendermint to shutdown at a given height.
This could be use by an external manager process that oversees upgrades by
checking out and installing new software versions and restarting the process. It
would subscribe to the relevant upgrade event (needs to be implemented) and call `/unsafe_stop` at
the correct height (of course only after getting approval from its user!)

## Status

Accepted

## Consequences

### Positive

- Make Tendermint and application versions native to the ABCI to more clearly communicate about them
- Distinguish clearly between protocol versions and software version to facilitate implementations in other languages
- Versions included in key data structures in easy to discern way
- Allows proposers to signal for upgrades and apps to decide when to actually change the version (and start signalling for a new version)

### Neutral

- Unclear how to version the initial P2P handshake itself
- Versions aren't being used (yet) to restrict peer compatibility
- Signaling for a new version happens through the proposer and must be tallied/tracked in the app.

### Negative

- Adds more fields to the ABCI
- Implies that a single codebase must be able to handle multiple versions
