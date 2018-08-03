# Protocol Versions

## TODO

- How do we want App to signal to Tendermint to update AppVersion? EndBlock tags
  or new ProposeTx msg ?
- How to handle peers syncing from old heights across AppVersion changes
- How to / should we version the authenticated encryption handshake itself (ie.
  upfront protocol negotiation for the P2PVersion)

## Changelog

- 28-07-2018: Updates from review
    - split into two ADRs - one for protocol, one for chains
    - include signalling for upgrades in header
- 16-07-2018: Initial draft - was originally joint ADR for protocol and chain
versions

## Context

The Software Version is covered by SemVer and described elsewhere.
It is not relevant to the protocol description, suffice to say that if any protocol version
changes, the software version changes, but not necessarily vice versa.

Software version shoudl be included in NodeInfo for convenience/diagnostics.

We are also interested in versioning across different blockchains in a
meaningful way, for instance to differentiate branches of a contentious
hard-fork. We leave that for a later ADR.

Here we focus on protocol versions.

## Requirements

We need to version components of the blockchain that may be independently upgraded.
We need to do it in a way that is scalable and maintainable - we can't just litter
the code with conditionals.

We can consider the complete version of the protocol to contain the following sub-versions:
BlockVersion, P2PVersion, AppVersion. These versions reflect the major sub-components
of the software that are likely to evolve together, at different rates, and in different ways,
as described below.

The BlockVersion defines the core of the blockchain data structures and
should change infrequently.

The P2PVersion defines how peers connect and communicate with eachother - it's
not part of the blockchain data structures, but defines the protocols used to build the
blockchain. It may change gradually.

The AppVersion ensures we only connect to peers that will compute the same App
state root. Tendermint otherwise doesn't care about the AppVersion, but it helps
to make it a native field for observability sake.

### BlockVersion

- All tendermint hashed data-structures (headers, votes, txs, responses, etc.).
	- Note the semantic meaning of a transaction may change according to the AppVersion,
		but the way txs are merklized into the header is part of the BlockVersion
- It should be the least frequent/likely to change.
	- Tendermint should be stabilizing - it's just Atomic Broadcast.
	- We can start considering for Tendermint v2.0 in a year
- It's easy to determine the version of a block from its serialized form

### P2PVersion

- All p2p and reactor messaging (messages, detectable behaviour)
- Will change gradually as reactors evolve to improve performance and support new features
	- eg proposed new message types BatchTx in the mempool and HasBlockPart in the consensus
- It's easy to determine the version of a peer from its first serialized message/s
- New versions must be compatible with at least one old version to allow gradual upgrades

### AppVersion

- The ABCI state machine (txs, begin/endblock behaviour, commit hashing)
- Behaviour and message types will change abruptly in the course of the life of a chain
- Need to minimize complexity of the code for supporting different AppVersions at different heights
- Ideally, each version of the software supports only a *single* AppVersion at one time
    - this means we checkout different versions of the software at different heights instead of littering the code
          with conditionals
    - minimize the number of data migrations required across AppVersion (ie. most AppVersion should be able to read the same state from disk as previous AppVersion).
- NOTE: we still need to support peers syncing from old heights across AppVersion changes
    - this is a non-trivial and difficult point (!)

## Ideal

Each component of the software is independently versioned in a modular way and its easy to mix and match and upgrade.

Good luck pal ;)

## Proposal

Each of BlockVersion, AppVersion, P2PVersion is a monotonically increasing int64.

To use these versions, we need to update the block Header, the p2p NodeInfo, and the ABCI.

### Header

Block Header should include a `Version` struct as its first field like:

```
type Version struct {
    BlockVersion ProtocolVersion

    ChainID string
    AppVersion ProtocolVersion
}

type ProtocolVersion struct {
    Current int64
    Next int64
}
```

Note this effectively makes BlockVersion the first field in the block Header.
Since we have settled on a proto3 header, the ability to read the BlockVersion out of the serialized header is unanimous.

Using a Version struct gives us more flexibility to add fields without breaking
the header.

The ProtocolVersion struct lets block proposer's specify both the current
Block and App versions, as well as proposed next versions.


### NodeInfo

NodeInfo should include a Version struct as its first field like:

```
type Version struct {
    P2PVersion int64

    ChainID string
    BlockVersion int64
    AppVersion int64
    SoftwareVersion string
}
```

Note this effectively makes P2PVersion the first field in the NodeInfo, so it
should be easy to read this out of the serialized header if need be to facilitate an upgrade.

The SoftwareVersion here should include the name of the software client and
it's SemVer version - this is for convenience only. The other versions and
ChainID will determine peer compatibility (described below).


### ABCI

RequestInfo should add support for protocol versions like:

```
message RequestInfo {
  string software_version
  int64 block_version
  int64 p2p_version
}
```

Similarly, ResponseInfo should return the versions:

```
message ResponseInfo {
  string data

  string software_version
  int64 app_version

  int64 last_block_height
  bytes last_block_app_hash
}
```

TODO: we need some way for the app to tell Tendermint it wants to signal for a
Next AppVersion. Options:

- use new ProposeTx ABCI msg for this
- use tags in ResponseEndBlock and incorporate into Proposal, eg.
    - KVPair: `("next_app_version", <new AppVersion>)`
    - Could also include a tag for the height the upgrade should happen at or by

### BlockVersion

BlockVersion is included in both the Header and the NodeInfo.

Only connect to peers with the same or higher BlockVersion.

Changing BlockVersion should happen quite infrequently and ideally only for extreme emergency.

Note Ethereum has not had to make an upgrade like this (everything has been at state machine level, AFAIK).

### P2PVersion

P2PVersion is not included in the block Header, just the NodeInfo.

P2PVersion is the first field in the NodeInfo. NodeInfo is also proto3 so this is easy to read out.

Each P2PVersion must be compatible with at least one previous version. For each P2PVersion, we keep a list of the previous
versions it is compatible with.

Note we need the peer/reactor protocols to take the versions of peers into account when sending messages:

- don't send messages they don't understand
- don't send messages they don't expect

Doing this will be specific to the upgrades being made.

Note we also include the list of reactor channels in the NodeInfo and already don't send messages for channels the peer doesn't understand.
If upgrades always use new channels, this simplifies the development cost of backwards compatibility.

Note NodeInfo is only exchanged after the authenticated encryption handshake to ensure that it's private.
Doing any version exchange before encrypting could be considered information leakage, though I'm not sure
how much that matters compared to being able to upgrade the protocol.

XXX: if needed, can we change the meaning of the first byte of the first message to encode a handshake version?
this is the first byte of a 32-byte ed25519 pubkey.

### AppVersion

AppVersion is also included in the block Header and the NodeInfo.

AppVersion essentially defines how the AppHash is computed. Since peers with
different AppVersion will likely compute different AppHash for blocks,
we only maintain connections to peers with the correct AppVersion for the height they're at.
That is, we only connect to current peers with the same AppVersion, and to old peers that have the right AppVersion for where they
are in the syncing process.

Note this requires all validators to upgrade their versions at exactly the required height.

### Tendermint Changes

Compatibility requirements at the peer layer have already been described.

It may be valuable to support an `/unsafe_stop?height=_` endpoint to tell Tendermint to shutdown at a given height.
This could be use by an external manager process that oversees upgrades by
checking out and installing new software versions and restarting the process. It
would subscribe to the relevant upgrade event tags and call `/unsafe_stop` at
the correct height (of course only after getting approval from its user!)
