---
order: 2
title: Applications
---

# Applications

Please ensure you've first read the spec for [ABCI Methods and Types](abci.md)

Here we cover the following components of ABCI applications:

- [Connection State](#connection-state) - the interplay between ABCI connections and application state
  and the differences between `CheckTx` and `DeliverTx`.
- [Transaction Results](#transaction-results) - rules around transaction
  results and validity
- [Validator Set Updates](#updating-the-validator-set) - how validator sets are
  changed during `InitChain` and `EndBlock`
- [Query](#query) - standards for using the `Query` method and proofs about the
  application state
- [Crash Recovery](#crash-recovery) - handshake protocol to synchronize
  Tendermint and the application on startup.
- [State Sync](#state-sync) - rapid bootstrapping of new nodes by restoring state machine snapshots

## Connection State

Since Tendermint maintains four concurrent ABCI connections, it is typical
for an application to maintain a distinct state for each, and for the states to
be synchronized during `Commit`.

### Concurrency

In principle, each of the four ABCI connections operate concurrently with one
another. This means applications need to ensure access to state is
thread safe. In practice, both the
[default in-process ABCI client](https://github.com/tendermint/tendermint/blob/v0.34.4/abci/client/local_client.go#L18)
and the
[default Go ABCI
server](https://github.com/tendermint/tendermint/blob/v0.34.4/abci/server/socket_server.go#L32)
use global locks across all connections, so they are not
concurrent at all. This means if your app is written in Go, and compiled in-process with Tendermint
using the default `NewLocalClient`, or run out-of-process using the default `SocketServer`,
ABCI messages from all connections will be linearizable (received one at a
time).

The existence of this global mutex means Go application developers can get
thread safety for application state by routing *all* reads and writes through the ABCI
system. Thus it may be *unsafe* to expose application state directly to an RPC
interface, and unless explicit measures are taken, all queries should be routed through the ABCI Query method.

### BeginBlock

The BeginBlock request can be used to run some code at the beginning of
every block. It also allows Tendermint to send the current block hash
and header to the application, before it sends any of the transactions.

The app should remember the latest height and header (ie. from which it
has run a successful Commit) so that it can tell Tendermint where to
pick up from when it restarts. See information on the Handshake, below.

### Commit

Application state should only be persisted to disk during `Commit`.

Before `Commit` is called, Tendermint locks and flushes the mempool so that no new messages will
be received on the mempool connection. This provides an opportunity to safely update all four connection
states to the latest committed state at once.

When `Commit` completes, it unlocks the mempool.

WARNING: if the ABCI app logic processing the `Commit` message sends a
`/broadcast_tx_sync` or `/broadcast_tx_commit` and waits for the response
before proceeding, it will deadlock. Executing those `broadcast_tx` calls
involves acquiring a lock that is held during the `Commit` call, so it's not
possible. If you make the call to the `broadcast_tx` endpoints concurrently,
that's no problem, it just can't be part of the sequential logic of the
`Commit` function.

### Consensus Connection

The Consensus Connection should maintain a `DeliverTxState` - the working state
for block execution. It should be updated by the calls to `BeginBlock`, `DeliverTx`,
and `EndBlock` during block execution and committed to disk as the "latest
committed state" during `Commit`.

Updates made to the `DeliverTxState` by each method call must be readable by each subsequent method -
ie. the updates are linearizable.

### Mempool Connection

The mempool Connection should maintain a `CheckTxState`
to sequentially process pending transactions in the mempool that have
not yet been committed. It should be initialized to the latest committed state
at the end of every `Commit`.

Before calling `Commit`, Tendermint will lock and flush the mempool connection,
ensuring that all existing CheckTx are responded to and no new ones can begin.
The `CheckTxState` may be updated concurrently with the `DeliverTxState`, as
messages may be sent concurrently on the Consensus and Mempool connections.

After `Commit`, while still holding the mempool lock, CheckTx is run again on all transactions that remain in the
node's local mempool after filtering those included in the block.
An additional `Type` parameter is made available to the CheckTx function that
indicates whether an incoming transaction is new (`CheckTxType_New`), or a
recheck (`CheckTxType_Recheck`).

Finally, after re-checking transactions in the mempool, Tendermint will unlock
the mempool connection. New transactions are once again able to be processed through CheckTx.

Note that CheckTx is just a weak filter to keep invalid transactions out of the block chain.
CheckTx doesn't have to check everything that affects transaction validity; the
expensive things can be skipped.  It's weak because a Byzantine node doesn't
care about CheckTx; it can propose a block full of invalid transactions if it wants.

#### Replay Protection

To prevent old transactions from being replayed, CheckTx must implement
replay protection.

It is possible for old transactions to be sent to the application. So
it is important CheckTx implements some logic to handle them.

### Query Connection

The Info Connection should maintain a `QueryState` for answering queries from the user,
and for initialization when Tendermint first starts up (both described further
below).
It should always contain the latest committed state associated with the
latest committed block.

`QueryState` should be set to the latest `DeliverTxState` at the end of every `Commit`,
after the full block has been processed and the state committed to disk.
Otherwise it should never be modified.

Tendermint Core currently uses the Query connection to filter peers upon
connecting, according to IP address or node ID. For instance,
returning non-OK ABCI response to either of the following queries will
cause Tendermint to not connect to the corresponding peer:

- `p2p/filter/addr/<ip addr>`, where `<ip addr>` is an IP address.
- `p2p/filter/id/<id>`, where `<is>` is the hex-encoded node ID (the hash of
  the node's p2p pubkey).

Note: these query formats are subject to change!

### Snapshot Connection

The Snapshot Connection is optional, and is only used to serve state sync snapshots for other nodes
and/or restore state sync snapshots to a local node being bootstrapped.

For more information, see [the state sync section of this document](#state-sync).

## Transaction Results

The `Info` and `Log` fields are non-deterministic values for debugging/convenience purposes
that are otherwise ignored.

The `Data` field must be strictly deterministic, but can be arbitrary data.

### Gas

Ethereum introduced the notion of `gas` as an abstract representation of the
cost of resources used by nodes when processing transactions. Every operation in the
Ethereum Virtual Machine uses some amount of gas, and gas can be accepted at a market-variable price.
Users propose a maximum amount of gas for their transaction; if the tx uses less, they get
the difference credited back. Tendermint adopts a similar abstraction,
though uses it only optionally and weakly, allowing applications to define
their own sense of the cost of execution.

In Tendermint, the
[ConsensusParams.Block.MaxGas](../../proto/tendermint/types/params.proto)
limits the amount of `gas` that can be used in a block.  The default value is
`-1`, meaning no limit, or that the concept of gas is meaningless.

Responses contain a `GasWanted` and `GasUsed` field. The former is the maximum
amount of gas the sender of a tx is willing to use, and the later is how much it actually
used. Applications should enforce that `GasUsed <= GasWanted` - ie. tx execution
should halt before it can use more resources than it requested.

When `MaxGas > -1`, Tendermint enforces the following rules:

- `GasWanted <= MaxGas` for all txs in the mempool
- `(sum of GasWanted in a block) <= MaxGas` when proposing a block

If `MaxGas == -1`, no rules about gas are enforced.

Note that Tendermint does not currently enforce anything about Gas in the consensus, only the mempool.
This means it does not guarantee that committed blocks satisfy these rules!
It is the application's responsibility to return non-zero response codes when gas limits are exceeded.

The `GasUsed` field is ignored completely by Tendermint. That said, applications should enforce:

- `GasUsed <= GasWanted` for any given transaction
- `(sum of GasUsed in a block) <= MaxGas` for every block

In the future, we intend to add a `Priority` field to the responses that can be
used to explicitly prioritize txs in the mempool for inclusion in a block
proposal. See [#1861](https://github.com/tendermint/tendermint/issues/1861).

### CheckTx

If `Code != 0`, it will be rejected from the mempool and hence
not broadcasted to other peers and not included in a proposal block.

`Data` contains the result of the CheckTx transaction execution, if any. It is
semantically meaningless to Tendermint.

### DeliverTx

DeliverTx is the workhorse of the blockchain. Tendermint sends the
DeliverTx requests asynchronously but in order, and relies on the
underlying socket protocol (ie. TCP) to ensure they are received by the
app in order. They have already been ordered in the global consensus by
the Tendermint protocol.

If DeliverTx returns `Code != 0`, the transaction will be considered invalid,
though it is still included in the block.

DeliverTx also returns a [Code, Data, and Log](../../proto/tendermint/abci/types.proto#L189-L191).

`Data` contains the result of the CheckTx transaction execution, if any. It is
semantically meaningless to Tendermint.

Both the `Code` and `Data` are included in a structure that is hashed into the
`LastResultsHash` of the next block header.

`Events` include any events for the execution, which Tendermint will use to index
the transaction by. This allows transactions to be queried according to what
events took place during their execution.

## Updating the Validator Set

The application may set the validator set during InitChain, and may update it during
EndBlock.

Note that the maximum total power of the validator set is bounded by
`MaxTotalVotingPower = MaxInt64 / 8`. Applications are responsible for ensuring
they do not make changes to the validator set that cause it to exceed this
limit.

Additionally, applications must ensure that a single set of updates does not contain any duplicates -
a given public key can only appear once within a given update. If an update includes
duplicates, the block execution will fail irrecoverably.

### InitChain

The `InitChain` method can return a list of validators.
If the list is empty, Tendermint will use the validators loaded in the genesis
file.
If the list returned by `InitChain` is not empty, Tendermint will use its contents as the validator set.
This way the application can set the initial validator set for the
blockchain.

### EndBlock

Updates to the Tendermint validator set can be made by returning
`ValidatorUpdate` objects in the `ResponseEndBlock`:

```protobuf
message ValidatorUpdate {
  tendermint.crypto.keys.PublicKey pub_key
  int64 power
}

message PublicKey {
  oneof {
    ed25519 bytes = 1;
  }
```

The `pub_key` currently supports only one type:

- `type = "ed25519"`

The `power` is the new voting power for the validator, with the
following rules:

- power must be non-negative
- if power is 0, the validator must already exist, and will be removed from the
  validator set
- if power is non-0:
    - if the validator does not already exist, it will be added to the validator
    set with the given power
    - if the validator does already exist, its power will be adjusted to the given power
- the total power of the new validator set must not exceed MaxTotalVotingPower

Note the updates returned in block `H` will only take effect at block `H+2`.

## Consensus Parameters

ConsensusParams enforce certain limits in the blockchain, like the maximum size
of blocks, amount of gas used in a block, and the maximum acceptable age of
evidence. They can be set in InitChain and updated in EndBlock.

### BlockParams.MaxBytes

The maximum size of a complete Protobuf encoded block.
This is enforced by Tendermint consensus.

This implies a maximum transaction size that is this MaxBytes, less the expected size of
the header, the validator set, and any included evidence in the block.

Must have `0 < MaxBytes < 100 MB`.

### BlockParams.MaxGas

The maximum of the sum of `GasWanted` that will be allowed in a proposed block.
This is *not* enforced by Tendermint consensus.
It is left to the app to enforce (ie. if txs are included past the
limit, they should return non-zero codes). It is used by Tendermint to limit the
txs included in a proposed block.

Must have `MaxGas >= -1`.
If `MaxGas == -1`, no limit is enforced.

### BlockParams.RecheckTx

This indicates whether all nodes in the network should perform a `CheckTx` on all
transactions remaining in the mempool directly *after* the execution of every block,
i.e. whenever a new application state is created. This is often useful for garbage
collection.

The change will come into effect immediately after `FinalizeBlock` has been
called.

This was previously a local mempool config parameter.

### EvidenceParams.MaxAgeDuration

This is the maximum age of evidence in time units.
This is enforced by Tendermint consensus.

If a block includes evidence older than this (AND the evidence was created more
than `MaxAgeNumBlocks` ago), the block will be rejected (validators won't vote
for it).

Must have `MaxAgeDuration > 0`.

### EvidenceParams.MaxAgeNumBlocks

This is the maximum age of evidence in blocks.
This is enforced by Tendermint consensus.

If a block includes evidence older than this (AND the evidence was created more
than `MaxAgeDuration` ago), the block will be rejected (validators won't vote
for it).

Must have `MaxAgeNumBlocks > 0`.

### EvidenceParams.MaxNum

This is the maximum number of evidence that can be committed to a single block.

The product of this and the `MaxEvidenceBytes` must not exceed the size of
a block minus it's overhead ( ~ `MaxBytes`).

Must have `MaxNum > 0`.

### SynchronyParams.Precision

`SynchronyParams.Precision` is a parameter of the Proposer-Based Timestamps algorithm.
that configures the acceptable upper-bound of clock drift among
all of the nodes on a Tendermint network. Any two nodes on a Tendermint network
are expected to have clocks that differ by at most `Precision`.

### SynchronyParams.MessageDelay

`SynchronyParams.MessageDelay` is a parameter of the Proposer-Based Timestamps
algorithm that configures the acceptable upper-bound for transmitting a `Proposal`
message from the proposer to all of the validators on the network.

### Updates

The application may set the ConsensusParams during InitChain, and update them during
EndBlock. If the ConsensusParams is empty, it will be ignored. Each field
that is not empty will be applied in full. For instance, if updating the
Block.MaxBytes, applications must also set the other Block fields (like
Block.MaxGas), even if they are unchanged, as they will otherwise cause the
value to be updated to 0.

#### InitChain

ResponseInitChain includes a ConsensusParams.
If ConsensusParams is nil, Tendermint will use the params loaded in the genesis
file. If ConsensusParams is not nil, Tendermint will use it.
This way the application can determine the initial consensus params for the
blockchain.

#### EndBlock

ResponseEndBlock includes a ConsensusParams.
If ConsensusParams nil, Tendermint will do nothing.
If ConsensusParam is not nil, Tendermint will use it.
This way the application can update the consensus params over time.

Note the updates returned in block `H` will take effect right away for block
`H+1`.

## Query

Query is a generic method with lots of flexibility to enable diverse sets
of queries on application state. Tendermint makes use of Query to filter new peers
based on ID and IP, and exposes Query to the user over RPC.

Note that calls to Query are not replicated across nodes, but rather query the
local node's state - hence they may return stale reads. For reads that require
consensus, use a transaction.

The most important use of Query is to return Merkle proofs of the application state at some height
that can be used for efficient application-specific light-clients.

Note Tendermint has technically no requirements from the Query
message for normal operation - that is, the ABCI app developer need not implement
Query functionality if they do not wish too.

### Query Proofs

The Tendermint block header includes a number of hashes, each providing an
anchor for some type of proof about the blockchain. The `ValidatorsHash` enables
quick verification of the validator set, the `DataHash` gives quick
verification of the transactions included in the block, etc.

The `AppHash` is unique in that it is application specific, and allows for
application-specific Merkle proofs about the state of the application.
While some applications keep all relevant state in the transactions themselves
(like Bitcoin and its UTXOs), others maintain a separated state that is
computed deterministically *from* transactions, but is not contained directly in
the transactions themselves (like Ethereum contracts and accounts).
For such applications, the `AppHash` provides a much more efficient way to verify light-client proofs.

ABCI applications can take advantage of more efficient light-client proofs for
their state as follows:

- return the Merkle root of the deterministic application state in
`ResponseCommit.Data`. This Merkle root will be included as the `AppHash` in the next block.
- return efficient Merkle proofs about that application state in `ResponseQuery.Proof`
  that can be verified using the `AppHash` of the corresponding block.

For instance, this allows an application's light-client to verify proofs of
absence in the application state, something which is much less efficient to do using the block hash.

Some applications (eg. Ethereum, Cosmos-SDK) have multiple "levels" of Merkle trees,
where the leaves of one tree are the root hashes of others. To support this, and
the general variability in Merkle proofs, the `ResponseQuery.Proof` has some minimal structure:

```protobuf
message ProofOps {
  repeated ProofOp ops
}

message ProofOp {
  string type = 1;
  bytes key = 2;
  bytes data = 3;
}
```

Each `ProofOp` contains a proof for a single key in a single Merkle tree, of the specified `type`.
This allows ABCI to support many different kinds of Merkle trees, encoding
formats, and proofs (eg. of presence and absence) just by varying the `type`.
The `data` contains the actual encoded proof, encoded according to the `type`.
When verifying the full proof, the root hash for one ProofOp is the value being
verified for the next ProofOp in the list. The root hash of the final ProofOp in
the list should match the `AppHash` being verified against.

### Peer Filtering

When Tendermint connects to a peer, it sends two queries to the ABCI application
using the following paths, with no additional data:

- `/p2p/filter/addr/<IP:PORT>`, where `<IP:PORT>` denote the IP address and
  the port of the connection
- `p2p/filter/id/<ID>`, where `<ID>` is the peer node ID (ie. the
  pubkey.Address() for the peer's PubKey)

If either of these queries return a non-zero ABCI code, Tendermint will refuse
to connect to the peer.

### Paths

Queries are directed at paths, and may optionally include additional data.

The expectation is for there to be some number of high level paths
differentiating concerns, like `/p2p`, `/store`, and `/app`. Currently,
Tendermint only uses `/p2p`, for filtering peers. For more advanced use, see the
implementation of
[Query in the Cosmos-SDK](https://github.com/cosmos/cosmos-sdk/blob/v0.23.1/baseapp/baseapp.go#L333).

## Crash Recovery

On startup, Tendermint calls the `Info` method on the Info Connection to get the latest
committed state of the app. The app MUST return information consistent with the
last block it succesfully completed Commit for.

If the app succesfully committed block H, then `last_block_height = H` and `last_block_app_hash = <hash returned by Commit for block H>`. If the app
failed during the Commit of block H, then `last_block_height = H-1` and
`last_block_app_hash = <hash returned by Commit for block H-1, which is the hash in the header of block H>`.

We now distinguish three heights, and describe how Tendermint syncs itself with
the app.

```md
storeBlockHeight = height of the last block Tendermint saw a commit for
stateBlockHeight = height of the last block for which Tendermint completed all
    block processing and saved all ABCI results to disk
appBlockHeight = height of the last block for which ABCI app succesfully
    completed Commit

```

Note we always have `storeBlockHeight >= stateBlockHeight` and `storeBlockHeight >= appBlockHeight`
Note also Tendermint never calls Commit on an ABCI app twice for the same height.

The procedure is as follows.

First, some simple start conditions:

If `appBlockHeight == 0`, then call InitChain.

If `storeBlockHeight == 0`, we're done.

Now, some sanity checks:

If `storeBlockHeight < appBlockHeight`, error
If `storeBlockHeight < stateBlockHeight`, panic
If `storeBlockHeight > stateBlockHeight+1`, panic

Now, the meat:

If `storeBlockHeight == stateBlockHeight && appBlockHeight < storeBlockHeight`,
replay all blocks in full from `appBlockHeight` to `storeBlockHeight`.
This happens if we completed processing the block, but the app forgot its height.

If `storeBlockHeight == stateBlockHeight && appBlockHeight == storeBlockHeight`, we're done.
This happens if we crashed at an opportune spot.

If `storeBlockHeight == stateBlockHeight+1`
This happens if we started processing the block but didn't finish.

If `appBlockHeight < stateBlockHeight`
    replay all blocks in full from `appBlockHeight` to `storeBlockHeight-1`,
    and replay the block at `storeBlockHeight` using the WAL.
This happens if the app forgot the last block it committed.

If `appBlockHeight == stateBlockHeight`,
    replay the last block (storeBlockHeight) in full.
This happens if we crashed before the app finished Commit

If `appBlockHeight == storeBlockHeight`
    update the state using the saved ABCI responses but dont run the block against the real app.
This happens if we crashed after the app finished Commit but before Tendermint saved the state.

## State Sync

A new node joining the network can simply join consensus at the genesis height and replay all
historical blocks until it is caught up. However, for large chains this can take a significant
amount of time, often on the order of days or weeks.

State sync is an alternative mechanism for bootstrapping a new node, where it fetches a snapshot
of the state machine at a given height and restores it. Depending on the application, this can
be several orders of magnitude faster than replaying blocks.

Note that state sync does not currently backfill historical blocks, so the node will have a
truncated block history - users are advised to consider the broader network implications of this in
terms of block availability and auditability. This functionality may be added in the future.

For details on the specific ABCI calls and types, see the [methods and types section](abci.md).

### Taking Snapshots

Applications that want to support state syncing must take state snapshots at regular intervals. How
this is accomplished is entirely up to the application. A snapshot consists of some metadata and
a set of binary chunks in an arbitrary format:

- `Height (uint64)`: The height at which the snapshot is taken. It must be taken after the given
  height has been committed, and must not contain data from any later heights.

- `Format (uint32)`: An arbitrary snapshot format identifier. This can be used to version snapshot
  formats, e.g. to switch from Protobuf to MessagePack for serialization. The application can use
  this when restoring to choose whether to accept or reject a snapshot.

- `Chunks (uint32)`: The number of chunks in the snapshot. Each chunk contains arbitrary binary
  data, and should be less than 16 MB; 10 MB is a good starting point.

- `Hash ([]byte)`: An arbitrary hash of the snapshot. This is used to check whether a snapshot is
  the same across nodes when downloading chunks.

- `Metadata ([]byte)`: Arbitrary snapshot metadata, e.g. chunk hashes for verification or any other
  necessary info.

For a snapshot to be considered the same across nodes, all of these fields must be identical. When
sent across the network, snapshot metadata messages are limited to 4 MB.

When a new node is running state sync and discovering snapshots, Tendermint will query an existing
application via the ABCI `ListSnapshots` method to discover available snapshots, and load binary
snapshot chunks via `LoadSnapshotChunk`. The application is free to choose how to implement this
and which formats to use, but must provide the following guarantees:

- **Consistent:** A snapshot must be taken at a single isolated height, unaffected by
  concurrent writes. This can be accomplished by using a data store that supports ACID
  transactions with snapshot isolation.

- **Asynchronous:** Taking a snapshot can be time-consuming, so it must not halt chain progress,
  for example by running in a separate thread.

- **Deterministic:** A snapshot taken at the same height in the same format must be identical
  (at the byte level) across nodes, including all metadata. This ensures good availability of
  chunks, and that they fit together across nodes.

A very basic approach might be to use a datastore with MVCC transactions (such as RocksDB),
start a transaction immediately after block commit, and spawn a new thread which is passed the
transaction handle. This thread can then export all data items, serialize them using e.g.
Protobuf, hash the byte stream, split it into chunks, and store the chunks in the file system
along with some metadata - all while the blockchain is applying new blocks in parallel.

A more advanced approach might include incremental verification of individual chunks against the
chain app hash, parallel or batched exports, compression, and so on.

Old snapshots should be removed after some time - generally only the last two snapshots are needed
(to prevent the last one from being removed while a node is restoring it).

### Bootstrapping a Node

An empty node can be state synced by setting the configuration option `statesync.enabled =
true`. The node also needs the chain genesis file for basic chain info, and configuration for
light client verification of the restored snapshot: a set of Tendermint RPC servers, and a
trusted header hash and corresponding height from a trusted source, via the `statesync`
configuration section.

Once started, the node will connect to the P2P network and begin discovering snapshots. These
will be offered to the local application via the `OfferSnapshot` ABCI method. Once a snapshot
is accepted Tendermint will fetch and apply the snapshot chunks. After all chunks have been
successfully applied, Tendermint verifies the app's `AppHash` against the chain using the light
client, then switches the node to normal consensus operation.

#### Snapshot Discovery

When the empty node join the P2P network, it asks all peers to report snapshots via the
`ListSnapshots` ABCI call (limited to 10 per node). After some time, the node picks the most
suitable snapshot (generally prioritized by height, format, and number of peers), and offers it
to the application via `OfferSnapshot`. The application can choose a number of responses,
including accepting or rejecting it, rejecting the offered format, rejecting the peer who sent
it, and so on. Tendermint will keep discovering and offering snapshots until one is accepted or
the application aborts.

#### Snapshot Restoration

Once a snapshot has been accepted via `OfferSnapshot`, Tendermint begins downloading chunks from
any peers that have the same snapshot (i.e. that have identical metadata fields). Chunks are
spooled in a temporary directory, and then given to the application in sequential order via
`ApplySnapshotChunk` until all chunks have been accepted.

The method for restoring snapshot chunks is entirely up to the application.

During restoration, the application can respond to `ApplySnapshotChunk` with instructions for how
to continue. This will typically be to accept the chunk and await the next one, but it can also
ask for chunks to be refetched (either the current one or any number of previous ones), P2P peers
to be banned, snapshots to be rejected or retried, and a number of other responses - see the ABCI
reference for details.

If Tendermint fails to fetch a chunk after some time, it will reject the snapshot and try a
different one via `OfferSnapshot` - the application can choose whether it wants to support
restarting restoration, or simply abort with an error.

#### Snapshot Verification

Once all chunks have been accepted, Tendermint issues an `Info` ABCI call to retrieve the
`LastBlockAppHash`. This is compared with the trusted app hash from the chain, retrieved and
verified using the light client. Tendermint also checks that `LastBlockHeight` corresponds to the
height of the snapshot.

This verification ensures that an application is valid before joining the network. However, the
snapshot restoration may take a long time to complete, so applications may want to employ additional
verification during the restore to detect failures early. This might e.g. include incremental
verification of each chunk against the app hash (using bundled Merkle proofs), checksums to
protect against data corruption by the disk or network, and so on. However, it is important to
note that the only trusted information available is the app hash, and all other snapshot metadata
can be spoofed by adversaries.

Apps may also want to consider state sync denial-of-service vectors, where adversaries provide
invalid or harmful snapshots to prevent nodes from joining the network. The application can
counteract this by asking Tendermint to ban peers. As a last resort, node operators can use
P2P configuration options to whitelist a set of trusted peers that can provide valid snapshots.

#### Transition to Consensus

Once the snapshots have all been restored, Tendermint gathers additional information necessary for
bootstrapping the node (e.g. chain ID, consensus parameters, validator sets, and block headers)
from the genesis file and light client RPC servers. It also fetches and records the `AppVersion`
from the ABCI application.

Once the state machine has been restored and Tendermint has gathered this additional
information, it transitions to block sync (if enabled) to fetch any remaining blocks up the chain
head, and then transitions to regular consensus operation. At this point the node operates like
any other node, apart from having a truncated block history at the height of the restored snapshot.
