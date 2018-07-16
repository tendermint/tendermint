# Protocol and Chain Versions

The Software Version is covered by SemVer and described elsewhere.
It is not relevant to the protocol description, suffice to say that if any protocol version
changes, the software version changes. Software version is included in NodeInfo for convenience/diagnostics.

Here we focus on protocol and chain versions.

## Requirements

We need to version blockchains across protocols, networks, forks, etc.
We need to do it in a way that is scalable and maintainable - we can't just litter
the code with conditionals.
We need chain identifiers and descriptions so we can talk about a multitude of chains in a meaningful way.

Let's start with versions.

### Protocol Version

The complete version of the protocol contains the following sub-versions:
BlockVersion, P2PVersion, AppVersion. These versions reflect the major sub-components
of the software that are likely to evolve together, at different rates, and in different ways,
as described below.

#### BlockVersion

- All tendermint hashed data-structures (headers, votes, txs, responses, etc.).
	- Note the semantic meaning of a transaction may change according to the AppVersion,
		but the way txs are merklized into the header is part of the BlockVersion
- It should be the least frequent/likely to change.
	- Tendermint should be stabilizing - it's just Atomic Broadcast.
	- We can start considering for Tendermint v2.0 in a year
- For now, assume that BlockVersion will not change in the life of a given chain, except by critical emergency.
- To effectively change it, dump state and start a new blockchain with the new BlockVersion.
- It's easy to determine the version of a block from its serialized form

#### P2PVersion

- All peer messaging (messages, detectable behaviour)
- Will change gradually as reactors evolve to improve performance and support new features
	- eg proposed new message types BatchTx in the mempool and HasBlockPart in the consensus
- It's easy to determine the version of a peer from its first serialized message/s
- New versions must be compatible with at least one old version to allow gradual upgrades

#### AppVersion

- The ABCI state machine (txs, begin/endblock behaviour, commit hashing)
- Behaviour and message types will change abruptly in the course of the life of a chain
- Need to minimize complexity of the code for supporting different AppVersions at different heights
- Ideally, each version of the software supports only a *single* AppVersion at one time
    - this means we checkout different versions of the software at different heights instead of littering the code
          with conditionals
    - minimize the number of data migrations required across AppVersion (ie. most AppVersion should be able to read the same state from disk as previous AppVersion).
- NOTE: we still need to support peers syncing from old heights across AppVersion changes

### Networks

We need to support many independent networks running the same version of the software,
even possibly starting from the same initial state.
They must have distinct identifiers so that peers know which one they are joining and so
validators and users can prevent replay attacks.

Call this the `NetworkName`. It represents both the application being run and the community or intention
of running it.

Peers only connect to other peers with the same NetworkName.

### Forks

We need to support existing networks upgrading and forking, wherein they may do any of:
	- revert back to some height, continue with the same versions but new blocks
	- arbitrarily mutate state at some height, continue with the same versions (eg. Dao Fork)
	- change the AppVersion at some height

Note because of Tendermint's voting power threshold rules, a chain can only be extended under the "original" rules and under the new rules
if 1/3 or more is double signing, which is expressly prohibited, and is supposed to result in their punishment on both chains. Since they can censor
the punishment, the chain is expected to be hardforked to remove the validators. Thus, if both branches are to continue after a fork,
they will each require a new identifier, and the old chain identifier will be retired (ie. only useful for syncing history, not for new blocks)..

We need a consistent way to describe forks.

## Ideal

Each component of the software is independently versioned in a modular way and its easy to mix and match and upgrade.

Good luck pal ;)

## Proposal

### Versions

Each of BlockVersion, AppVersion, P2PVersion is a monotonically increasing int64.

An alternative might be to try to use Amino - ie. include the version number in the registration name for some type,
for instance the BlockVersion in the header. However this would require the registered-name of the header to change even
if it wasn't the header type itself that changed, which could be confusing. Thus, versions here will be integers in Amino encoded
structus. That said, Amino's flexibility (ie. both in registration of new types and proto3 extensibility in structs) will make
actual upgrades smoother.

#### BlockVersion

BlockVersion is the first field in the block Header. Since we have settled on a proto3 header,
the ability to read the BlockVersion out of the serialized header is unanimous.

BlockVersion is included in the NodeInfo and we only connect to peers with the same or higher BlockVersion.

In the event that we need to upgrade the core protocols and don't want to dump the state,
we can condition execution on the block version and use different handlers / message types as needed.
In such a case we may need to support connecting to old versions as well.

This isn't nice, but should happen quite infrequently and ideally only for extreme emergency.

Note Ethereum has not had to make an upgrade like this (everything has been at state machine level, AFAIK).

#### P2PVersion

P2PVersion is not included in the block Header.

P2PVersion is the first field in the NodeInfo. NodeInfo is also proto3 so this is easy to read out.

For each P2PVersion, we keep a list of previous versions that this one is compatible with so we can connect to
peers of any of those versions or higher.

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

#### AppVersion

AppVersion is also included in the block Header and the NodeInfo.

We only maintain connections to peers with the correct AppVersion for the height they're at.
That is, we only connect to current peers with the same AppVersion, and to old peers that have the right AppVersion for where they
are in the syncing process.

Note this requires all consensus nodes to upgrade their versions at exactly the required height.
Effectively doing so requires an external manager process that knows when the version should be updated and can co-ordinate shutting down,
upgrading and restarting at the correct height.

While this requires additional overhead in the form of a new manager process, it avoids the state machine logic getting
cluttered with conditionals to effect different rules at different heights.

More details on how this manager process works later below.

### ChainDescription

ChainDescription is a complete immutable description of a blockchain. It takes the following form:

```
ChainDescription = <NetworkName>/<BlockVersion>/<AppVersion>/<StateHash>/<ValHash>/<ConsensusParamsHash>
```

Here, StateHash is the merkle root of the initial state, ValHash is the merkle root of the initial Tendermint validator set,
and ConsensusParamsHash is the merkle root of the initial Tendermint consensus parameters.

The `genesis.json` file must contain enough information to compute this value. It need not contain the StateHash or ValHash itself,
but contain the state from which they can be computed with the given protocol versions.

XXX: should we split NetworkName into NetworkName and AppName?

#### ChainID

Define `ChainID = TMHASH(ChainDescriptor)`. It's the unique ID of a blockchain.

It should be Bech32 encoded when handled by users, eg. with `cosmoschain` prefix.

#### Forks and Uprades

When a chain forks or upgrades but continues the same history, it takes a new ChainDescription as follows:

```
ChainDescription = <ChainID>/x/<Height>/<ForkDescription>
```

Where
    - ChainID is the ChainID from the previous ChainDescription (ie. its hash)
    - `x` denotes that a change occured
    - `Height` is the height the change occured
    - ForkDescription has the same form as ChainDescription but for the fork
        - this allows forks to specify new versions for tendermint or the app, as well as arbitrary changes to the state or validator set

### ABCI Changes

On RequestInfo, Tendermint should send: TMSoftwareVersion, BlockVersion, AppVersion
On ResponseInfo, App should send: AppSoftwareVersion, AppVersion

For the App to signal an upgrade, it should use the following tags (KVPairs) in ResponseEndBlock:

- `("upgrade_chain_description", <new ChainDescription>)`
- `("upgrade_chain_height", <height to upgrade>)`

This provides a simple manner for applications to signal to external processes that they want to upgrade, and what height to do it at.

By using the ChainDescription, the app signals a summary of all changes expected to be made and when and exactly how the ChainDescription will change.

### Tendermint Changes

Compatibility requirements at the peer layer have already been described.

The only other change necessary is an `/unsafe_stop?height=_` endpoint to tell Tendermint to shutdown at a given height.
This is to enable it to be stopped at the height signalled by `upgrade_chain_height` and to then be restarted using the new ChainDescription (ie. new app).
Note if the ValidatorHash or ConsensusParamsHash were force-changed, Tendermint will require the new information.


### Process Manager

A simple process managers subscribes to the `upgrade_chain` events and calls `/unsafe_stop` for the given height.
Note before calling /unsafe_stop it must get approval from its user!
