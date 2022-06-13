# Upgrading Tendermint Core

This guide provides instructions for upgrading to specific versions of Tendermint Core.

## v0.36

### ABCI Changes

### ResponseCheckTx Parameter Change

`ResponseCheckTx` had fields that are not used by Tendermint, they are now removed.
In 0.36, we removed the following fields, from `ResponseCheckTx`: `Log`, `Info`, `Events`,
 `GasUsed` and `MempoolError`. 
`MempoolError` was used to signal to operators that a transaction was rejected from the mempool
by Tendermint itself. Right now, we return a regular error when this happens.

#### ABCI++

For information on how ABCI++ works, see the
[Specification](https://github.com/tendermint/tendermint/blob/master/spec/abci%2B%2B/README.md).
In particular, the simplest way to upgrade your application is described
[here](https://github.com/tendermint/tendermint/blob/master/spec/abci%2B%2B/abci++_tmint_expected_behavior_002_draft.md#adapting-existing-applications-that-use-abci).

#### Moving the `app_hash` parameter

The Application's hash (or any data representing the Application's current
state) is known by the time `FinalizeBlock` finishes its execution.
Accordingly, the `app_hash` parameter has been moved from `ResponseCommit` to
`ResponseFinalizeBlock`, since it makes sense for the Application to return
this value as soon as is it known.

#### ABCI Mutex

In previous versions of ABCI, Tendermint was prevented from making
concurrent calls to ABCI implementations by virtue of mutexes in the
implementation of Tendermint's ABCI infrastructure. These mutexes have
been removed from the current implementation and applications will now
be responsible for managing their own concurrency control.

To replicate the prior semantics, ensure that ABCI applications have a
single mutex that protects all ABCI method calls from concurrent
access. You can relax these requirements if your application can
provide safe concurrent access via other means. This safety is an
application concern so be very sure to test the application thoroughly
using realistic workloads and the race detector to ensure your
applications remains correct.

### Config Changes

- We have added a new, experimental tool to help operators migrate
  configuration files created by previous versions of Tendermint.
  To try this tool, run:

  ```shell
  # Install the tool.
  go install github.com/tendermint/tendermint/scripts/confix@latest

  # Run the tool with the old configuration file as input.
  # Replace the -config argument with your path.
  confix -config ~/.tendermint/config/config.toml -out updated.toml
  ```

  This tool should be able to update configurations from v0.34 and v0.35.  We
  plan to extend it to handle older configuration files in the future. For now,
  it will report an error (without making any changes) if it does not recognize
  the version that created the file.

- The default configuration for a newly-created node now disables indexing for
  ABCI event metadata. Existing node configurations that already have indexing
  turned on are not affected. Operators who wish to enable indexing for a new
  node, however, must now edit the `config.toml` explicitly.

- The function of seed nodes was modified in the past release. Now, seed nodes
  are treated identically to any other peer, however they only run the PEX
  reactor. Because of this `seeds` has been removed from the config. Users
  should add any seed nodes in the list of `bootstrap-peers`.

### RPC Changes

Tendermint v0.36 adds a new RPC event subscription API. The existing event
subscription API based on websockets is now deprecated. It will continue to
work throughout the v0.36 release, but the `subscribe`, `unsubscribe`, and
`unsubscribe_all` methods, along with websocket support, will be removed in
Tendermint v0.37.  Callers currently using these features should migrate as
soon as is practical to the new API.

To enable the new API, node operators set a new `event-log-window-size`
parameter in the `[rpc]` section of the `config.toml` file. This defines a
duration of time during which the node will log all events published to the
event bus for use by RPC consumers.

Consumers use the new `events` JSON-RPC method to poll for events matching
their query in the log. Unlike the streaming API, events are not discarded if
the caller is slow, loses its connection, or crashes. As long as the client
recovers before its events expire from the log window, it will be able to
replay and catch up after recovering. Also unlike the streaming API, the client
can tell if it has truly missed events because they have expired from the log.

The `events` method is a normal JSON-RPC method, and does not require any
non-standard response processing (in contrast with the old `subscribe`).
Clients can modify their query at any time, and no longer need to coordinate
subscribe and unsubscribe calls to handle multiple queries.

The Go client implementations in the Tendermint Core repository have all been
updated to add a new `Events` method, including the light client proxy.

A new `rpc/client/eventstream` package has also been added to make it easier
for users to update existing use of the streaming API to use the polling API
The `eventstream` package handles polling and delivers matching events to a
callback.

For more detailed information, see [ADR 075](https://tinyurl.com/adr075) which
defines and describes the new API in detail.

#### BroadcastTx Methods

All callers should use the new `broadcast_tx` method, which has the
same semantics as the legacy `broadcast_tx_sync` method. The
`broadcast_tx_sync` and `broadcast_tx_async` methods are now
deprecated and will be removed in 0.37.

Additionally the `broadcast_tx_commit` method is *also* deprecated,
and will be removed in 0.37. Client code that submits a transaction
and needs to wait for it to be committed to the chain, should poll
the tendermint to observe the transaction in the committed state.

### Timeout Parameter Changes

Tendermint v0.36 updates how the Tendermint consensus timing parameters are
configured. These parameters, `timeout-propose`, `timeout-propose-delta`,
`timeout-prevote`, `timeout-prevote-delta`, `timeout-precommit`,
`timeout-precommit-delta`, `timeout-commit`, and `skip-timeout-commit`, were
previously configured in `config.toml`. These timing parameters have moved and
are no longer configured in the `config.toml` file. These parameters have been
migrated into the `ConsensusParameters`. Nodes with these parameters set in the
local configuration file will see a warning logged on startup indicating that
these parameters are no longer used.

These parameters have also been pared-down. There are no longer separate
parameters for both the `prevote` and `precommit` phases of Tendermint. The
separate `timeout-prevote` and `timeout-precommit` parameters have been merged
into a single `timeout-vote` parameter that configures both of these similar
phases of the consensus protocol.

A set of reasonable defaults have been put in place for these new parameters
that will take effect when the node starts up in version v0.36. New chains
created using v0.36 and beyond will be able to configure these parameters in the
chain's `genesis.json` file. Chains that upgrade to v0.36 from a previous
compatible version of Tendermint will begin running with the default values.
Upgrading applications that wish to use different values from the defaults for
these parameters may do so by setting the `ConsensusParams.Timeout` field of the
`FinalizeBlock` `ABCI` response.

As a safety measure in case of unusual timing issues during the upgrade to
v0.36, an operator may override the consensus timeout values for a single node.
Note, however, that these overrides will be removed in Tendermint v0.37. See
[configuration](https://github.com/tendermint/tendermint/blob/master/docs/nodes/configuration.md)
for more information about these overrides.

For more discussion of this, see [ADR 074](https://tinyurl.com/adr074), which
lays out the reasoning for the changes as well as [RFC
009](https://tinyurl.com/rfc009) for a discussion of the complexities of
upgrading consensus parameters.

### RecheckTx Parameter Change

`RecheckTx` was previously enabled by the `recheck` parameter in the mempool
section of the `config.toml`.
Setting it to true made Tendermint invoke another `CheckTx` ABCI call on
each transaction remaining in the mempool following the execution of a block.
Similar to the timeout parameter changes, this parameter makes more sense as a
network-wide coordinated variable so that applications can be written knowing
either all nodes agree on whether to run `RecheckTx`.

Applications can turn on `RecheckTx` by altering the `ConsensusParams` in the
`FinalizeBlock` ABCI response. 

### CLI Changes

The functionality around resetting a node has been extended to make it safer. The
`unsafe-reset-all` command has been replaced by a `reset` command with four
subcommands: `blockchain`, `peers`, `unsafe-signer` and `unsafe-all`.

- `tendermint reset blockchain`: Clears a node of all blocks, consensus state, evidence,
  and indexed transactions. NOTE: This command does not reset application state.
  If you need to rollback the last application state (to recover from application
  nondeterminism), see instead the `tendermint rollback` command.
- `tendermint reset peers`: Clears the peer store, which persists information on peers used
  by the networking layer. This can be used to get rid of stale addresses or to switch
  to a predefined set of static peers.
- `tendermint reset unsafe-signer`: Resets the watermark level of the PrivVal File signer
  allowing it to sign votes from the genesis height. This should only be used in testing as
  it can lead to the node double signing.
- `tendermint reset unsafe-all`: A summation of the other three commands. This will delete
  the entire `data` directory which may include application data as well.

### Go API Changes

#### `crypto` Package Cleanup

The `github.com/tendermint/tendermint/crypto/tmhash` package was removed
to improve clarity. Users are encouraged to use the standard library
`crypto/sha256` package directly. However, as a convenience, some constants
and one function have moved to the Tendermint `crypto` package:

- The `crypto.Checksum` function returns the sha256 checksum of a
  byteslice. This is a wrapper around `sha256.Sum265` from the
  standard libary, but provided as a function to ease type
  requirements (the library function returns a `[32]byte` rather than
  a `[]byte`).
- `tmhash.TruncatedSize` is now `crypto.AddressSize` which was
  previously an alias for the same value.
- `tmhash.Size` and `tmhash.BlockSize` are now `crypto.HashSize` and
  `crypto.HashSize`.
- `tmhash.SumTruncated` is now available via `crypto.AddressHash` or by
  `crypto.Checksum(<...>)[:crypto.AddressSize]`

## v0.35

### ABCI Changes

* Added `AbciVersion` to `RequestInfo`. Applications should check that the ABCI version they expect is being used in order to avoid unimplemented changes errors.
* The method `SetOption` has been removed from the ABCI.Client interface. This feature was used in the early ABCI implementation's.
* Messages are written to a byte stream using uin64 length delimiters instead of int64.
* When mempool `v1` is enabled, transactions broadcasted via `sync` mode may return a successful
  response with a transaction hash indicating that the transaction was successfully inserted into
  the mempool. While this is true for `v0`, the `v1` mempool reactor may at a later point in time
  evict or even drop this transaction after a hash has been returned. Thus, the user or client must
  query for that transaction to check if it is still in the mempool.

### Config Changes

* The configuration file field `[fastsync]` has been renamed to `[blocksync]`.

* The top level configuration file field `fast-sync` has moved under the new `[blocksync]`
  field as `blocksync.enable`.

* `blocksync.version = "v1"` and `blocksync.version = "v2"` (previously `fastsync`)
  are no longer supported. Please use `v0` instead. During the v0.35 release cycle, `v0` was
  determined to suit the existing needs and the cost of maintaining the `v1` and `v2` modules
  was determined to be greater than necessary.


* All config parameters are now hyphen-case (also known as kebab-case) instead of snake_case. Before restarting the node make sure
  you have updated all the variables in your `config.toml` file.

* Added `--mode` flag and `mode` config variable on `config.toml` for setting Mode of the Node: `full` | `validator` | `seed` (default: `full`)
  [ADR-52](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-052-tendermint-mode.md)

* `BootstrapPeers` has been added as part of the new p2p stack. This will eventually replace
  `Seeds`. Bootstrap peers are connected with on startup if needed for peer discovery. Unlike
  persistent peers, there's no gaurantee that the node will remain connected with these peers.

* configuration values starting with `priv-validator-` have moved to the new
  `priv-validator` section, without the `priv-validator-` prefix.

* The fast sync process as well as the blockchain package and service has all
  been renamed to block sync

### Database Key Format Changes

The format of all tendermint on-disk database keys changes in
0.35. Upgrading nodes must either re-sync all data or run a migration
script provided in this release.

The script located in
`github.com/tendermint/tendermint/scripts/keymigrate/migrate.go` provides the
function `Migrate(context.Context, db.DB)` which you can operationalize as
makes sense for your deployment.

For ease of use the `tendermint` command includes a CLI version of the
migration script, which you can invoke, as in:

	tendermint key-migrate

This reads the configuration file as normal and allows the `--db-backend` and
`--db-dir` flags to override the database location as needed.

The migration operation is intended to be idempotent, and should be safe to
rerun on the same database multiple times.  As a safety measure, however, we
recommend that operators test out the migration on a copy of the database
first, if it is practical to do so, before applying it to the production data.

### CLI Changes

* You must now specify the node mode (validator|full|seed) in `tendermint init [mode]`

* The `--fast-sync` command line option has been renamed to `--blocksync.enable`

* If you had previously used `tendermint gen_node_key` to generate a new node
  key, keep in mind that it no longer saves the output to a file. You can use
  `tendermint init validator` or pipe the output of `tendermint gen_node_key` to
  `$TMHOME/config/node_key.json`:

  ```
  $ tendermint gen_node_key > $TMHOME/config/node_key.json
  ```

* CLI commands and flags are all now hyphen-case instead of snake_case.
  Make sure to adjust any scripts that calls a cli command with snake_casing

### API Changes

The p2p layer was reimplemented as part of the 0.35 release cycle and
all reactors were refactored to accomodate the change. As part of that work these
implementations moved into the `internal` package and are no longer
considered part of the public Go API of tendermint. These packages
are:

- `p2p`
- `mempool`
- `consensus`
- `statesync`
- `blockchain`
- `evidence`

Accordingly, the `node` package changed to reduce access to
tendermint internals: applications that use tendermint as a library
will need to change to accommodate these changes. Most notably:

- The `Node` type has become internal, and all constructors return a
  `service.Service` implementation.

- The `node.DefaultNewNode` and `node.NewNode` constructors are no
  longer exported and have been replaced with `node.New` and
  `node.NewDefault` which provide more functional interfaces.

To access any of the functionality previously available via the
`node.Node` type, use the `*local.Local` "RPC" client, that exposes
the full RPC interface provided as direct function calls. Import the
`github.com/tendermint/tendermint/rpc/client/local` package and pass
the node service as in the following:

```go
logger := log.NewNopLogger()

// Construct and start up a node with default settings.
node := node.NewDefault(logger)

// Construct a local (in-memory) RPC client to the node.
client := local.New(logger, node.(local.NodeService))
```

### gRPC Support

Mark gRPC in the RPC layer as deprecated and to be removed in 0.36.

### Peer Management Interface

When running with the new P2P Layer, the methods `UnsafeDialSeeds` and
`UnsafeDialPeers` RPC methods will always return an error. They are
deprecated and will be removed in 0.36 when the legacy peer stack is
removed.

Additionally the format of the Peer list returned in the `NetInfo`
method changes in this release to accommodate the different way that
the new stack tracks data about peers. This change affects users of
both stacks.

### Using the updated p2p library

The P2P library was reimplemented in this release. The new implementation is
enabled by default in this version of Tendermint. The legacy implementation is still
included in this version of Tendermint as a backstop to work around unforeseen
production issues. The new and legacy version are interoperable. If necessary,
you can enable the legacy implementation in the server configuration file.

To make use of the legacy P2P implemementation add or update the following field of
your server's configuration file under the `[p2p]` section:

```toml
[p2p]
...
use-legacy = true
...
```

If you need to do this, please consider filing an issue in the Tendermint repository
to let us know why. We plan to remove the legacy P2P code in the next (v0.36) release.

#### New p2p queue types

The new p2p implementation enables selection of the queue type to be used for
passing messages between peers.

The following values may be used when selecting which queue type to use:

* `fifo`: (**default**) An unbuffered and lossless queue that passes messages through
in the order in which they were received.

* `priority`: A priority queue of messages.

* `wdrr`: A queue implementing the Weighted Deficit Round Robin algorithm. A
weighted deficit round robin queue is created per peer. Each queue contains a
separate 'flow' for each of the channels of communication that exist between any two
peers. Tendermint maintains a channel per message type between peers. Each WDRR
queue maintains a shared buffered with a fixed capacity through which messages on different
flows are passed.
For more information on WDRR scheduling, see: https://en.wikipedia.org/wiki/Deficit_round_robin

To select a queue type, add or update the following field under the `[p2p]`
section of your server's configuration file.

```toml
[p2p]
...
queue-type = wdrr
...
```


### Support for Custom Reactor and Mempool Implementations

The changes to p2p layer removed existing support for custom
reactors. Based on our understanding of how this functionality was
used, the introduction of the prioritized mempool covers nearly all of
the use cases for custom reactors. If you are currently running custom
reactors and mempools and are having trouble seeing the migration path
for your project please feel free to reach out to the Tendermint Core
development team directly.

## v0.34.0

**Upgrading to Tendermint 0.34 requires a blockchain restart.**
This release is not compatible with previous blockchains due to changes to
the encoding format (see "Protocol Buffers," below) and the block header (see "Blockchain Protocol").

Note also that Tendermint 0.34 also requires Go 1.16 or higher.

### ABCI Changes

* The `ABCIVersion` is now `0.17.0`.

* New ABCI methods (`ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, and `ApplySnapshotChunk`)
  were added to support the new State Sync feature.
  Previously, syncing a new node to a preexisting network could take days; but with State Sync,
  new nodes are able to join a network in a matter of seconds.
  Read [the spec](https://github.com/tendermint/tendermint/blob/master/spec/abci/apps.md)
  if you want to learn more about State Sync, or if you'd like your application to use it.
  (If you don't want to support State Sync in your application, you can just implement these new
  ABCI methods as no-ops, leaving them empty.)

* `KV.Pair` has been replaced with `abci.EventAttribute`. The `EventAttribute.Index` field
  allows ABCI applications to dictate which events should be indexed.

* The blockchain can now start from an arbitrary initial height,
  provided to the application via `RequestInitChain.InitialHeight`.

* ABCI evidence type is now an enum with two recognized types of evidence:
  `DUPLICATE_VOTE` and `LIGHT_CLIENT_ATTACK`.
  Applications should be able to handle these evidence types
  (i.e., through slashing or other accountability measures).

* The [`PublicKey` type](https://github.com/tendermint/tendermint/blob/master/proto/tendermint/crypto/keys.proto#L13-L15)
  (used in ABCI as part of `ValidatorUpdate`) now uses a `oneof` protobuf type.
  Note that since Tendermint only supports ed25519 validator keys, there's only one
  option in the `oneof`.  For more, see "Protocol Buffers," below.

* The field `Proof`, on the ABCI type `ResponseQuery`, is now named `ProofOps`.
  For more, see "Crypto," below.

* The method `SetOption` has been removed from the ABCI.Client interface. This feature was used in the early ABCI implementation's.

### P2P Protocol

The default codec is now proto3, not amino. The schema files can be found in the `/proto`
directory. For more, see "Protobuf," below.

### Blockchain Protocol

* `Header#LastResultsHash`, which is the root hash of a Merkle tree built from
`ResponseDeliverTx(Code, Data)` as of v0.34 also includes `GasWanted` and `GasUsed`
fields.

* Merkle hashes of empty trees previously returned nothing, but now return the hash of an empty input,
  to conform with [RFC-6962](https://tools.ietf.org/html/rfc6962).
  This mainly affects `Header#DataHash`, `Header#LastResultsHash`, and
  `Header#EvidenceHash`, which are often empty. Non-empty hashes can also be affected, e.g. if their
  inputs depend on other (empty) Merkle hashes, giving different results.

### Transaction Indexing

Tendermint now relies on the application to tell it which transactions to index. This means that
in the `config.toml`, generated by Tendermint, there is no longer a way to specify which
transactions to index. `tx.height` and `tx.hash` will always be indexed when using the `kv` indexer.

Applications must now choose to either a) enable indexing for all transactions, or
b) allow node operators to decide which transactions to index.
Applications can notify Tendermint to index a specific transaction by setting
`Index: bool` to `true` in the Event Attribute:

```go
[]types.Event{
	{
		Type: "app",
		Attributes: []types.EventAttribute{
			{Key: []byte("creator"), Value: []byte("Cosmoshi Netowoko"), Index: true},
		},
	},
}
```

### Protocol Buffers

Tendermint 0.34 replaces Amino with Protocol Buffers for encoding.
This migration is extensive and results in a number of changes, however,
Tendermint only uses the types generated from Protocol Buffers for disk and
wire serialization.
**This means that these changes should not affect you as a Tendermint user.**

However, Tendermint users and contributors may note the following changes:

* Directory layout changes: All proto files have been moved under one directory, `/proto`.
  This is in line with the recommended file layout by [Buf](https://buf.build).
  For more, see the [Buf documentation](https://buf.build/docs/lint-checkers#file_layout).
* ABCI Changes: As noted in the "ABCI Changes" section above, the `PublicKey` type now uses
  a `oneof` type.

For more on the Protobuf changes, please see our [blog post on this migration](https://medium.com/tendermint/tendermint-0-34-protocol-buffers-and-you-8c40558939ae).

### Consensus Parameters

Tendermint 0.34 includes new and updated consensus parameters.

#### Version Parameters (New)

* `AppVersion`, which is the version of the ABCI application.

#### Evidence Parameters

* `MaxBytes`, which caps the total amount of evidence. The default is 1048576 (1 MB).

### Crypto

#### Keys

* Keys no longer include a type prefix. For example, ed25519 pubkeys have been renamed from
  `PubKeyEd25519` to `PubKey`. This reduces stutter (e.g., `ed25519.PubKey`).
* Keys are now byte slices (`[]byte`) instead of byte arrays (`[<size>]byte`).
* The multisig functionality that was previously in Tendermint now has
  a new home within the Cosmos SDK:
  [`cosmos/cosmos-sdk/types/multisig`](https://github.com/cosmos/cosmos-sdk/blob/master/crypto/types/multisig/multisignature.go).

#### `merkle` Package

* `SimpleHashFromMap()` and `SimpleProofsFromMap()` were removed.
* The prefix `Simple` has been removed. (For example, `SimpleProof` is now called `Proof`.)
* All protobuf messages have been moved to the `/proto` directory.
* The protobuf message `Proof` that contained multiple ProofOp's has been renamed to `ProofOps`.
  As noted above, this affects the ABCI type `ResponseQuery`:
  The field that was named Proof is now named `ProofOps`.
* `HashFromByteSlices` and `ProofsFromByteSlices` now return a hash for empty inputs, to conform with
  [RFC-6962](https://tools.ietf.org/html/rfc6962).

### `libs` Package

The `bech32` package has moved to the Cosmos SDK:
[`cosmos/cosmos-sdk/types/bech32`](https://github.com/cosmos/cosmos-sdk/tree/4173ea5ebad906dd9b45325bed69b9c655504867/types/bech32).

### CLI

The `tendermint lite` command has been renamed to `tendermint light` and has a slightly different API.

### Light Client

We have a new, rewritten light client! You can
[read more](https://medium.com/tendermint/everything-you-need-to-know-about-the-tendermint-light-client-f80d03856f98)
about the justifications and details behind this change.

Other user-relevant changes include:

* The old `lite` package was removed; the new light client uses the `light` package.
* The `Verifier` was broken up into two pieces:
	* Core verification logic (pure `VerifyX` functions)
	* `Client` object, which represents the complete light client
* The new light clients stores headers & validator sets as `LightBlock`s
* The RPC client can be found in the `/rpc` directory.
* The HTTP(S) proxy is located in the `/proxy` directory.

### `state` Package

* A new field `State.InitialHeight` has been added to record the initial chain height, which must be `1`
  (not `0`) if starting from height `1`. This can be configured via the genesis field `initial_height`.
* The `state` package now has a `Store` interface. All functions in
  [state/store.go](https://github.com/tendermint/tendermint/blob/56911ee35298191c95ef1c7d3d5ec508237aaff4/state/store.go#L42-L42)
  are now part of the interface. The interface returns errors on all methods and can be used by calling `state.NewStore(dbm.DB)`.

### `privval` Package

All requests are now accompanied by the chain ID from the network.
This is a optional field and can be ignored by key management systems;
however, if you are using the same key management system for multiple different
blockchains, we recommend that you check the chain ID.


### RPC

* `/unsafe_start_cpu_profiler`, `/unsafe_stop_cpu_profiler` and
  `/unsafe_write_heap_profile` were removed.
   For profiling, please use the pprof server, which can
  be enabled through `--rpc.pprof_laddr=X` flag or `pprof_laddr=X` config setting
  in the rpc section.
* The `Content-Type` header returned on RPC calls is now (correctly) set as `application/json`.

### Version

Version is now set through Go linker flags `ld_flags`. Applications that are using tendermint as a library should set this at compile time.

Example:

```sh
go install -mod=readonly -ldflags "-X github.com/tendermint/tendermint/version.TMCoreSemVer=$(go list -m github.com/tendermint/tendermint | sed  's/ /\@/g') -s -w " -trimpath ./cmd
```

Additionally, the exported constant `version.Version` is now `version.TMCoreSemVer`.

## v0.33.4

### Go API

* `rpc/client` HTTP and local clients have been moved into `http` and `local`
  subpackages, and their constructors have been renamed to `New()`.

### Protobuf Changes

When upgrading to version 0.33.4 you will have to fetch the `third_party`
directory along with the updated proto files.

### Block Retention

ResponseCommit added a field for block retention. The application can provide information to Tendermint on how to prune blocks.
If an application would like to not prune any blocks pass a `0` in this field.

```proto
message ResponseCommit {
  // reserve 1
  bytes  data          = 2; // the Merkle root hash
  ++ uint64 retain_height = 3; // the oldest block height to retain ++
}
```

## v0.33.0

This release is not compatible with previous blockchains due to commit becoming
signatures only and fields in the header have been removed.

### Blockchain Protocol

`TotalTxs` and `NumTxs` were removed from the header. `Commit` now consists
mostly of just signatures.

```go
type Commit struct {
	Height     int64
	Round      int
	BlockID    BlockID
	Signatures []CommitSig
}
```

```go
type BlockIDFlag byte

const (
	// BlockIDFlagAbsent - no vote was received from a validator.
	BlockIDFlagAbsent BlockIDFlag = 0x01
	// BlockIDFlagCommit - voted for the Commit.BlockID.
	BlockIDFlagCommit = 0x02
	// BlockIDFlagNil - voted for nil.
	BlockIDFlagNil = 0x03
)

type CommitSig struct {
	BlockIDFlag      BlockIDFlag
	ValidatorAddress Address
	Timestamp        time.Time
	Signature        []byte
}
```

See [\#63](https://github.com/tendermint/spec/pull/63) for the complete spec
change.

### P2P Protocol

The secret connection now includes a transcript hashing. If you want to
implement a handshake (or otherwise have an existing implementation), you'll
need to make the same changes that were made
[here](https://github.com/tendermint/tendermint/pull/3668).

### Config Changes

You will need to generate a new config if you have used a prior version of tendermint.

Tags have been entirely renamed throughout the codebase to events and there
keys are called
[compositeKeys](https://github.com/tendermint/tendermint/blob/6d05c531f7efef6f0619155cf10ae8557dd7832f/docs/app-dev/indexing-transactions.md).

Evidence Params has been changed to include duration.

* `consensus_params.evidence.max_age_duration`.
* Renamed `consensus_params.evidence.max_age` to `max_age_num_blocks`.

### Go API

* `libs/common` has been removed in favor of specific pkgs.
	* `async`
	* `service`
	* `rand`
	* `net`
	* `strings`
	* `cmap`
* removal of `errors` pkg

### RPC Changes

* `/validators` is now paginated (default: 30 vals per page)
* `/block_results` response format updated [see RPC docs for details](https://docs.tendermint.com/master/rpc/#/Info/block_results)
* Event suffix has been removed from the ID in event responses
* IDs are now integers not `json-client-XYZ`

## v0.32.0

This release is compatible with previous blockchains,
however the new ABCI Events mechanism may create some complexity
for nodes wishing to continue operation with v0.32 from a previous version.
There are some minor breaking changes to the RPC.

### Config Changes

If you have `db_backend` set to `leveldb` in your config file, please change it
to `goleveldb` or `cleveldb`.

### RPC Changes

The default listen address for the RPC is now `127.0.0.1`. If you want to expose
it publicly, you have to explicitly configure it. Note exposing the RPC to the
public internet may not be safe - endpoints which return a lot of data may
enable resource exhaustion attacks on your node, causing the process to crash.

Any consumers of `/block_results` need to be mindful of the change in all field
names from CamelCase to Snake case, eg. `results.DeliverTx` is now `results.deliver_tx`.
This is a fix, but it's breaking.

### ABCI Changes

ABCI responses which previously had a `Tags` field now have an `Events` field
instead. The original `Tags` field was simply a list of key-value pairs, where
each key effectively represented some attribute of an event occuring in the
blockchain, like `sender`, `receiver`, or `amount`. However, it was difficult to
represent the occurence of multiple events (for instance, multiple transfers) in a single list.
The new `Events` field contains a list of `Event`, where each `Event` is itself a list
of key-value pairs, allowing for more natural expression of multiple events in
eg. a single DeliverTx or EndBlock. Note each `Event` also includes a `Type`, which is meant to categorize the
event.

For transaction indexing, the index key is
prefixed with the event type: `{eventType}.{attributeKey}`.
If the same event type and attribute key appear multiple times, the values are
appended in a list.

To make queries, include the event type as a prefix. For instance if you
previously queried for `recipient = 'XYZ'`, and after the upgrade you name your event `transfer`,
the new query would be for `transfer.recipient = 'XYZ'`.

Note that transactions indexed on a node before upgrading to v0.32 will still be indexed
using the old scheme. For instance, if a node upgraded at height 100,
transactions before 100 would be queried with `recipient = 'XYZ'` and
transactions after 100 would be queried with `transfer.recipient = 'XYZ'`.
While this presents additional complexity to clients, it avoids the need to
reindex. Of course, you can reset the node and sync from scratch to re-index
entirely using the new scheme.

We illustrate further with a more complete example.

Prior to the update, suppose your `ResponseDeliverTx` look like:

```go
abci.ResponseDeliverTx{
  Tags: []kv.Pair{
	{Key: []byte("sender"), Value: []byte("foo")},
	{Key: []byte("recipient"), Value: []byte("bar")},
	{Key: []byte("amount"), Value: []byte("35")},
  }
}
```

The following queries would match this transaction:

```go
query.MustParse("tm.event = 'Tx' AND sender = 'foo'")
query.MustParse("tm.event = 'Tx' AND recipient = 'bar'")
query.MustParse("tm.event = 'Tx' AND sender = 'foo' AND recipient = 'bar'")
```

Following the upgrade, your `ResponseDeliverTx` would look something like:
the following `Events`:

```go
abci.ResponseDeliverTx{
  Events: []abci.Event{
	{
	  Type: "transfer",
	  Attributes: kv.Pairs{
		{Key: []byte("sender"), Value: []byte("foo")},
		{Key: []byte("recipient"), Value: []byte("bar")},
		{Key: []byte("amount"), Value: []byte("35")},
	  },
	}
}
```

Now the following queries would match this transaction:

```go
query.MustParse("tm.event = 'Tx' AND transfer.sender = 'foo'")
query.MustParse("tm.event = 'Tx' AND transfer.recipient = 'bar'")
query.MustParse("tm.event = 'Tx' AND transfer.sender = 'foo' AND transfer.recipient = 'bar'")
```

For further documentation on `Events`, see the [docs](https://github.com/tendermint/tendermint/blob/60827f75623b92eff132dc0eff5b49d2025c591e/docs/spec/abci/abci.md#events).

### Go Applications

The ABCI Application interface changed slightly so the CheckTx and DeliverTx
methods now take Request structs. The contents of these structs are just the raw
tx bytes, which were previously passed in as the argument.

## v0.31.6

There are no breaking changes in this release except Go API of p2p and
mempool packages. Hovewer, if you're using cleveldb, you'll need to change
the compilation tag:

Use `cleveldb` tag instead of `gcc` to compile Tendermint with CLevelDB or
use `make build_c` / `make install_c` (full instructions can be found at
<https://docs.tendermint.com/v0.35/introduction/install.html)

## v0.31.0

This release contains a breaking change to the behaviour of the pubsub system.
It also contains some minor breaking changes in the Go API and ABCI.
There are no changes to the block or p2p protocols, so v0.31.0 should work fine
with blockchains created from the v0.30 series.

### RPC

The pubsub no longer blocks on publishing. This may cause some WebSocket (WS) clients to stop working as expected.
If your WS client is not consuming events fast enough, Tendermint can terminate the subscription.
In this case, the WS client will receive an error with description:

```json
{
  "jsonrpc": "2.0",
  "id": "{ID}#event",
  "error": {
	"code": -32000,
	"msg": "Server error",
	"data": "subscription was canceled (reason: client is not pulling messages fast enough)" // or "subscription was canceled (reason: Tendermint exited)"
  }
}

Additionally, there are now limits on the number of subscribers and
subscriptions that can be active at once. See the new
`rpc.max_subscription_clients` and `rpc.max_subscriptions_per_client` values to
configure this.
```

### Applications

Simple rename of `ConsensusParams.BlockSize` to `ConsensusParams.Block`.

The `ConsensusParams.Block.TimeIotaMS` field was also removed. It's configured
in the ConsensusParsm in genesis.

### Go API

See the [CHANGELOG](CHANGELOG.md). These are relatively straight forward.

## v0.30.0

This release contains a breaking change to both the block and p2p protocols,
however it may be compatible with blockchains created with v0.29.0 depending on
the chain history. If your blockchain has not included any pieces of evidence,
or no piece of evidence has been included in more than one block,
and if your application has never returned multiple updates
for the same validator in a single block, then v0.30.0 will work fine with
blockchains created with v0.29.0.

The p2p protocol change is to fix the proposer selection algorithm again.
Note that proposer selection is purely a p2p concern right
now since the algorithm is only relevant during real time consensus.
This change is thus compatible with v0.29.0, but
all nodes must be upgraded to avoid disagreements on the proposer.

### Applications

Applications must ensure they do not return duplicates in
`ResponseEndBlock.ValidatorUpdates`. A pubkey must only appear once per set of
updates. Duplicates will cause irrecoverable failure. If you have a very good
reason why we shouldn't do this, please open an issue.

## v0.29.0

This release contains some breaking changes to the block and p2p protocols,
and will not be compatible with any previous versions of the software, primarily
due to changes in how various data structures are hashed.

Any implementations of Tendermint blockchain verification, including lite clients,
will need to be updated. For specific details:

* [Merkle tree](https://github.com/tendermint/spec/blob/master/spec/blockchain/encoding.md#merkle-trees)
* [ConsensusParams](https://github.com/tendermint/spec/blob/master/spec/blockchain/state.md#consensusparams)

There was also a small change to field ordering in the vote struct. Any
implementations of an out-of-process validator (like a Key-Management Server)
will need to be updated. For specific details:

* [Vote](https://github.com/tendermint/spec/blob/master/spec/consensus/signing.md#votes)

Finally, the proposer selection algorithm continues to evolve. See the
[work-in-progress
specification](https://github.com/tendermint/tendermint/pull/3140).

For everything else, please see the [CHANGELOG](./CHANGELOG.md#v0.29.0).

## v0.28.0

This release breaks the format for the `priv_validator.json` file
and the protocol used for the external validator process.
It is compatible with v0.27.0 blockchains (neither the BlockProtocol nor the
P2PProtocol have changed).

Please read carefully for details about upgrading.

**Note:** Backup your `config/priv_validator.json`
before proceeding.

### `priv_validator.json`

The `config/priv_validator.json` is now two files:
`config/priv_validator_key.json` and `data/priv_validator_state.json`.
The former contains the key material, the later contains the details on the last
message signed.

When running v0.28.0 for the first time, it will back up any pre-existing
`priv_validator.json` file and proceed to split it into the two new files.
Upgrading should happen automatically without problem.

To upgrade manually, use the provided `privValUpgrade.go` script, with exact paths for the old
`priv_validator.json` and the locations for the two new files. It's recomended
to use the default paths, of `config/priv_validator_key.json` and
`data/priv_validator_state.json`, respectively:

```sh
go run scripts/privValUpgrade.go <old-path> <new-key-path> <new-state-path>
```

### External validator signers

The Unix and TCP implementations of the remote signing validator
have been consolidated into a single implementation.
Thus in both cases, the external process is expected to dial
Tendermint. This is different from how Unix sockets used to work, where
Tendermint dialed the external process.

The `PubKeyMsg` was also split into separate `Request` and `Response` types
for consistency with other messages.

Note that the TCP sockets don't yet use a persistent key,
so while they're encrypted, they can't yet be properly authenticated.
See [#3105](https://github.com/tendermint/tendermint/issues/3105).
Note the Unix socket has neither encryption nor authentication, but will
add a shared-secret in [#3099](https://github.com/tendermint/tendermint/issues/3099).

## v0.27.0

This release contains some breaking changes to the block and p2p protocols,
but does not change any core data structures, so it should be compatible with
existing blockchains from the v0.26 series that only used Ed25519 validator keys.
Blockchains using Secp256k1 for validators will not be compatible. This is due
to the fact that we now enforce which key types validators can use as a
consensus param. The default is Ed25519, and Secp256k1 must be activated
explicitly.

It is recommended to upgrade all nodes at once to avoid incompatibilities at the
peer layer - namely, the heartbeat consensus message has been removed (only
relevant if `create_empty_blocks=false` or `create_empty_blocks_interval > 0`),
and the proposer selection algorithm has changed. Since proposer information is
never included in the blockchain, this change only affects the peer layer.

### Go API Changes

#### libs/db

The ReverseIterator API has changed the meaning of `start` and `end`.
Before, iteration was from `start` to `end`, where
`start > end`. Now, iteration is from `end` to `start`, where `start < end`.
The iterator also excludes `end`. This change allows a simplified and more
intuitive logic, aligning the semantic meaning of `start` and `end` in the
`Iterator` and `ReverseIterator`.

### Applications

This release enforces a new consensus parameter, the
ValidatorParams.PubKeyTypes. Applications must ensure that they only return
validator updates with the allowed PubKeyTypes. If a validator update includes a
pubkey type that is not included in the ConsensusParams.Validator.PubKeyTypes,
block execution will fail and the consensus will halt.

By default, only Ed25519 pubkeys may be used for validators. Enabling
Secp256k1 requires explicit modification of the ConsensusParams.
Please update your application accordingly (ie. restrict validators to only be
able to use Ed25519 keys, or explicitly add additional key types to the genesis
file).

## v0.26.0

This release contains a lot of changes to core data types and protocols. It is not
compatible to the old versions and there is no straight forward way to update
old data to be compatible with the new version.

To reset the state do:

```sh
tendermint unsafe_reset_all
```

Here we summarize some other notable changes to be mindful of.

### Config Changes

All timeouts must be changed from integers to strings with their duration, for
instance `flush_throttle_timeout = 100` would be changed to
`flush_throttle_timeout = "100ms"` and `timeout_propose = 3000` would be changed
to `timeout_propose = "3s"`.

### RPC Changes

The default behaviour of `/abci_query` has been changed to not return a proof,
and the name of the parameter that controls this has been changed from `trusted`
to `prove`. To get proofs with your queries, ensure you set `prove=true`.

Various version fields like `amino_version`, `p2p_version`, `consensus_version`,
and `rpc_version` have been removed from the `node_info.other` and are
consolidated under the tendermint semantic version (ie. `node_info.version`) and
the new `block` and `p2p` protocol versions under `node_info.protocol_version`.

### ABCI Changes

Field numbers were bumped in the `Header` and `ResponseInfo` messages to make
room for new `version` fields. It should be straight forward to recompile the
protobuf file for these changes.

#### Proofs

The `ResponseQuery.Proof` field is now structured as a `[]ProofOp` to support
generalized Merkle tree constructions where the leaves of one Merkle tree are
the root of another. If you don't need this functionality, and you used to
return `<proof bytes>` here, you should instead return a single `ProofOp` with
just the `Data` field set:

```go
[]ProofOp{
	ProofOp{
		Data: <proof bytes>,
	}
}
```

For more information, see:

* [ADR-026](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/docs/architecture/adr-026-general-merkle-proof.md)
* [Relevant ABCI
  documentation](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/docs/spec/abci/apps.md#query-proofs)
* [Description of
  keys](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/crypto/merkle/proof_key_path.go#L14)

### Go API Changes

#### crypto/merkle

The `merkle.Hasher` interface was removed. Functions which used to take `Hasher`
now simply take `[]byte`. This means that any objects being Merklized should be
serialized before they are passed in.

#### node

The `node.RunForever` function was removed. Signal handling and running forever
should instead be explicitly configured by the caller. See how we do it
[here](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/cmd/tendermint/commands/run_node.go#L60).

### Other

All hashes, except for public key addresses, are now 32-bytes.

## v0.25.0

This release has minimal impact.

If you use GasWanted in ABCI and want to enforce it, set the MaxGas in the genesis file (default is no max).

## v0.24.0

New 0.24.0 release contains a lot of changes to the state and types. It's not
compatible to the old versions and there is no straight forward way to update
old data to be compatible with the new version.

To reset the state do:

```sh
tendermint unsafe_reset_all
```

Here we summarize some other notable changes to be mindful of.

### Config changes

`p2p.max_num_peers` was removed in favor of `p2p.max_num_inbound_peers` and
`p2p.max_num_outbound_peers`.

```toml
# Maximum number of inbound peers
max_num_inbound_peers = 40

# Maximum number of outbound peers to connect to, excluding persistent peers
max_num_outbound_peers = 10
```

As you can see, the default ratio of inbound/outbound peers is 4/1. The reason
is we want it to be easier for new nodes to connect to the network. You can
tweak these parameters to alter the network topology.

### RPC Changes

The result of `/commit` used to contain `header` and `commit` fields at the top level. These are now contained under the `signed_header` field.

### ABCI Changes

The header has been upgraded and contains new fields, but none of the existing
fields were changed, except their order.

The `Validator` type was split into two, one containing an `Address` and one
containing a `PubKey`. When processing `RequestBeginBlock`, use the `Validator`
type, which contains just the `Address`. When returning `ResponseEndBlock`, use
the `ValidatorUpdate` type, which contains just the `PubKey`.

### Validator Set Updates

Validator set updates returned in ResponseEndBlock for height `H` used to take
effect immediately at height `H+1`. Now they will be delayed one block, to take
effect at height `H+2`. Note this means that the change will be seen by the ABCI
app in the `RequestBeginBlock.LastCommitInfo` at block `H+3`. Apps were already
required to maintain a map from validator addresses to pubkeys since v0.23 (when
pubkeys were removed from RequestBeginBlock), but now they may need to track
multiple validator sets at once to accomodate this delay.

### Block Size

The `ConsensusParams.BlockSize.MaxTxs` was removed in favour of
`ConsensusParams.BlockSize.MaxBytes`, which is now enforced. This means blocks
are limitted only by byte-size, not by number of transactions.
