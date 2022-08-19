# Methods and Types

## Overview

The ABCI message types are defined in a [protobuf
file](https://github.com/tendermint/tendermint/blob/master/proto/tendermint/abci/types.proto).

ABCI methods are split across four separate ABCI _connections_:

- Consensus connection: `InitChain`, `BeginBlock`, `DeliverTx`, `EndBlock`, `Commit`
- Mempool connection: `CheckTx`
- Info connection: `Info`, `Query`
- Snapshot connection: `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, `ApplySnapshotChunk`

The consensus connection is driven by a consensus protocol and is responsible
for block execution.

The mempool connection is for validating new transactions, before they're
shared or included in a block.

The info connection is for initialization and for queries from the user.

The snapshot connection is for serving and restoring [state sync snapshots](apps.md#state-sync).

Additionally, there is a `Flush` method that is called on every connection,
and an `Echo` method that is just for debugging.

More details on managing state across connections can be found in the section on
[ABCI Applications](apps.md).

## Errors

Some methods (`Echo, Info, InitChain, BeginBlock, EndBlock, Commit`),
don't return errors because an error would indicate a critical failure
in the application and there's nothing Tendermint can do. The problem
should be addressed and both Tendermint and the application restarted.

All other methods (`Query, CheckTx, DeliverTx`) return an
application-specific response `Code uint32`, where only `0` is reserved
for `OK`.

Finally, `Query`, `CheckTx`, and `DeliverTx` include a `Codespace string`, whose
intended use is to disambiguate `Code` values returned by different domains of the
application. The `Codespace` is a namespace for the `Code`.

## Events

Some methods (`CheckTx, BeginBlock, DeliverTx, EndBlock`)
include an `Events` field in their `Response*`. Each event contains a type and a
list of attributes, which are key-value pairs denoting something about what happened
during the method's execution.

Events can be used to index transactions and blocks according to what happened
during their execution. Note that the set of events returned for a block from
`BeginBlock` and `EndBlock` are merged. In case both methods return the same
tag, only the value defined in `EndBlock` is used.

Each event has a `type` which is meant to categorize the event for a particular
`Response*` or tx. A `Response*` or tx may contain multiple events with duplicate
`type` values, where each distinct entry is meant to categorize attributes for a
particular event. Every key and value in an event's attributes must be UTF-8
encoded strings along with the event type itself.

```protobuf
message Event {
  string                  type       = 1;
  repeated EventAttribute attributes = 2;
}
```

The attributes of an `Event` consist of a `key`, `value` and a `index`. The index field notifies the indexer within Tendermint to index the event. This field is non-deterministic and will vary across different nodes in the network.

```protobuf
message EventAttribute {
  bytes key   = 1;
  bytes value = 2;
  bool  index = 3;  // nondeterministic
}
```

Example:

```go
 abci.ResponseDeliverTx{
  // ...
 Events: []abci.Event{
  {
   Type: "validator.provisions",
   Attributes: []abci.EventAttribute{
    abci.EventAttribute{Key: []byte("address"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("amount"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("balance"), Value: []byte("..."), Index: true},
   },
  },
  {
   Type: "validator.provisions",
   Attributes: []abci.EventAttribute{
    abci.EventAttribute{Key: []byte("address"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("amount"), Value: []byte("..."), Index: false},
    abci.EventAttribute{Key: []byte("balance"), Value: []byte("..."), Index: false},
   },
  },
  {
   Type: "validator.slashed",
   Attributes: []abci.EventAttribute{
    abci.EventAttribute{Key: []byte("address"), Value: []byte("..."), Index: false},
    abci.EventAttribute{Key: []byte("amount"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("reason"), Value: []byte("..."), Index: true},
   },
  },
  // ...
 },
}
```

## EvidenceType

A part of Tendermint's security model is the use of evidence which serves as proof of
malicious behaviour by a network participant. It is the responsibility of Tendermint
to detect such malicious behaviour, to gossip this and commit it to the chain and once
verified by all validators to pass it on to the application through the ABCI. It is the
responsibility of the application then to handle the evidence and exercise punishment.

EvidenceType has the following protobuf format:

```proto
enum EvidenceType {
  UNKNOWN               = 0;
  DUPLICATE_VOTE        = 1;
  LIGHT_CLIENT_ATTACK   = 2;
}
```

There are two forms of evidence: Duplicate Vote and Light Client Attack. More
information can be found in either [data structures](https://github.com/tendermint/spec/blob/master/spec/core/data_structures.md)
or [accountability](https://github.com/tendermint/spec/blob/master/spec/light-client/accountability.md)

## Determinism

ABCI applications must implement deterministic finite-state machines to be
securely replicated by the Tendermint consensus. This means block execution
over the Consensus Connection must be strictly deterministic: given the same
ordered set of requests, all nodes will compute identical responses, for all
BeginBlock, DeliverTx, EndBlock, and Commit. This is critical, because the
responses are included in the header of the next block, either via a Merkle root
or directly, so all nodes must agree on exactly what they are.

For this reason, it is recommended that applications not be exposed to any
external user or process except via the ABCI connections to a consensus engine
like Tendermint Core. The application must only change its state based on input
from block execution (BeginBlock, DeliverTx, EndBlock, Commit), and not through
any other kind of request. This is the only way to ensure all nodes see the same
transactions and compute the same results.

If there is some non-determinism in the state machine, consensus will eventually
fail as nodes disagree over the correct values for the block header. The
non-determinism must be fixed and the nodes restarted.

Sources of non-determinism in applications may include:

- Hardware failures
    - Cosmic rays, overheating, etc.
- Node-dependent state
    - Random numbers
    - Time
- Underspecification
    - Library version changes
    - Race conditions
    - Floating point numbers
    - JSON serialization
    - Iterating through hash-tables/maps/dictionaries
- External Sources
    - Filesystem
    - Network calls (eg. some external REST API service)

See [#56](https://github.com/tendermint/abci/issues/56) for original discussion.

Note that some methods (`Query, CheckTx, DeliverTx`) return
explicitly non-deterministic data in the form of `Info` and `Log` fields. The `Log` is
intended for the literal output from the application's logger, while the
`Info` is any additional info that should be returned. These are the only fields
that are not included in block header computations, so we don't need agreement
on them. All other fields in the `Response*` must be strictly deterministic.

## Block Execution

The first time a new blockchain is started, Tendermint calls
`InitChain`. From then on, the following sequence of methods is executed for each
block:

`BeginBlock, [DeliverTx], EndBlock, Commit`

where one `DeliverTx` is called for each transaction in the block.
The result is an updated application state.
Cryptographic commitments to the results of DeliverTx, EndBlock, and
Commit are included in the header of the next block.

## State Sync

State sync allows new nodes to rapidly bootstrap by discovering, fetching, and applying
state machine snapshots instead of replaying historical blocks. For more details, see the
[state sync section](apps.md#state-sync).

When a new node is discovering snapshots in the P2P network, existing nodes will call
`ListSnapshots` on the application to retrieve any local state snapshots. The new node will
offer these snapshots to its local application via `OfferSnapshot`.

Once the application accepts a snapshot and begins restoring it, Tendermint will fetch snapshot
chunks from existing nodes via `LoadSnapshotChunk` and apply them sequentially to the local
application with `ApplySnapshotChunk`. When all chunks have been applied, the application
`AppHash` is retrieved via an `Info` query and compared to the blockchain's `AppHash` verified
via light client.

## Messages

### Echo

- **Request**:
    - `Message (string)`: A string to echo back
- **Response**:
    - `Message (string)`: The input string
- **Usage**:
    - Echo a string to test an abci client/server implementation

### Flush

- **Usage**:
    - Signals that messages queued on the client should be flushed to
    the server. It is called periodically by the client
    implementation to ensure asynchronous requests are actually
    sent, and is called immediately to make a synchronous request,
    which returns when the Flush response comes back.

### Info

- **Request**:
    - `Version (string)`: The Tendermint software semantic version
    - `BlockVersion (uint64)`: The Tendermint Block Protocol version
    - `P2PVersion (uint64)`: The Tendermint P2P Protocol version
- **Response**:
    - `Data (string)`: Some arbitrary information
    - `Version (string)`: The application software semantic version
    - `AppVersion (uint64)`: The application protocol version
    - `LastBlockHeight (int64)`: Latest block for which the app has
    called Commit
    - `LastBlockAppHash ([]byte)`: Latest result of Commit
- **Usage**:
    - Return information about the application state.
    - Used to sync Tendermint with the application during a handshake
    that happens on startup.
    - The returned `AppVersion` will be included in the Header of every block.
    - Tendermint expects `LastBlockAppHash` and `LastBlockHeight` to
    be updated during `Commit`, ensuring that `Commit` is never
    called twice for the same block height.

### InitChain

- **Request**:
    - `Time (google.protobuf.Timestamp)`: Genesis time.
    - `ChainID (string)`: ID of the blockchain.
    - `ConsensusParams (ConsensusParams)`: Initial consensus-critical parameters.
    - `Validators ([]ValidatorUpdate)`: Initial genesis validators, sorted by voting power.
    - `AppStateBytes ([]byte)`: Serialized initial application state. Amino-encoded JSON bytes.
    - `InitialHeight (int64)`: Height of the initial block (typically `1`).
- **Response**:
    - `ConsensusParams (ConsensusParams)`: Initial
    consensus-critical parameters (optional).
    - `Validators ([]ValidatorUpdate)`: Initial validator set (optional).
    - `AppHash ([]byte)`: Initial application hash.
- **Usage**:
    - Called once upon genesis.
    - If ResponseInitChain.Validators is empty, the initial validator set will be the RequestInitChain.Validators
    - If ResponseInitChain.Validators is not empty, it will be the initial
    validator set (regardless of what is in RequestInitChain.Validators).
    - This allows the app to decide if it wants to accept the initial validator
    set proposed by tendermint (ie. in the genesis file), or if it wants to use
    a different one (perhaps computed based on some application specific
    information in the genesis file).

### Query

- **Request**:
    - `Data ([]byte)`: Raw query bytes. Can be used with or in lieu
    of Path.
    - `Path (string)`: Path of request, like an HTTP GET path. Can be
    used with or in liue of Data.
        - Apps MUST interpret '/store' as a query by key on the
      underlying store. The key SHOULD be specified in the Data field.
        - Apps SHOULD allow queries over specific types like
      '/accounts/...' or '/votes/...'
    - `Height (int64)`: The block height for which you want the query
    (default=0 returns data for the latest committed block). Note
    that this is the height of the block containing the
    application's Merkle root hash, which represents the state as it
    was after committing the block at Height-1
    - `Prove (bool)`: Return Merkle proof with response if possible
- **Response**:
    - `Code (uint32)`: Response code.
    - `Log (string)`: The output of the application's logger. May
    be non-deterministic.
    - `Info (string)`: Additional information. May
    be non-deterministic.
    - `Index (int64)`: The index of the key in the tree.
    - `Key ([]byte)`: The key of the matching data.
    - `Value ([]byte)`: The value of the matching data.
    - `Proof (Proof)`: Serialized proof for the value data, if requested, to be
    verified against the `AppHash` for the given Height.
    - `Height (int64)`: The block height from which data was derived.
    Note that this is the height of the block containing the
    application's Merkle root hash, which represents the state as it
    was after committing the block at Height-1
    - `Codespace (string)`: Namespace for the `Code`.
- **Usage**:
    - Query for data from the application at current or past height.
    - Optionally return Merkle proof.
    - Merkle proof includes self-describing `type` field to support many types
    of Merkle trees and encoding formats.

### BeginBlock

- **Request**:
    - `Hash ([]byte)`: The block's hash. This can be derived from the
    block header.
    - `Header (struct{})`: The block header.
    - `LastCommitInfo (LastCommitInfo)`: Info about the last commit, including the
    round, and the list of validators and which ones signed the last block.
    - `ByzantineValidators ([]Evidence)`: List of evidence of
    validators that acted maliciously.
- **Response**:
    - `Events ([]abci.Event)`: Type & Key-Value events for indexing
- **Usage**:
    - Signals the beginning of a new block. Called prior to
    any DeliverTxs.
    - The header contains the height, timestamp, and more - it exactly matches the
    Tendermint block header. We may seek to generalize this in the future.
    - The `LastCommitInfo` and `ByzantineValidators` can be used to determine
    rewards and punishments for the validators. NOTE validators here do not
    include pubkeys.

### CheckTx

- **Request**:
    - `Tx ([]byte)`: The request transaction bytes
    - `Type (CheckTxType)`: What type of `CheckTx` request is this? At present,
    there are two possible values: `CheckTx_New` (the default, which says
    that a full check is required), and `CheckTx_Recheck` (when the mempool is
    initiating a normal recheck of a transaction).
- **Response**:
    - `Code (uint32)`: Response code
    - `Data ([]byte)`: Result bytes, if any.
    - `Log (string)`: The output of the application's logger. May
    be non-deterministic.
    - `Info (string)`: Additional information. May
    be non-deterministic.
    - `GasWanted (int64)`: Amount of gas requested for transaction.
    - `GasUsed (int64)`: Amount of gas consumed by transaction.
    - `Events ([]abci.Event)`: Type & Key-Value events for indexing
    transactions (eg. by account).
    - `Codespace (string)`: Namespace for the `Code`.
- **Usage**:
    - Technically optional - not involved in processing blocks.
    - Guardian of the mempool: every node runs CheckTx before letting a
    transaction into its local mempool.
    - The transaction may come from an external user or another node
    - CheckTx need not execute the transaction in full, but rather a light-weight
    yet stateful validation, like checking signatures and account balances, but
    not running code in a virtual machine.
    - Transactions where `ResponseCheckTx.Code != 0` will be rejected - they will not be broadcast to
    other nodes or included in a proposal block.
    - Tendermint attributes no other value to the response code

### DeliverTx

- **Request**:
    - `Tx ([]byte)`: The request transaction bytes.
- **Response**:
    - `Code (uint32)`: Response code.
    - `Data ([]byte)`: Result bytes, if any.
    - `Log (string)`: The output of the application's logger. May
    be non-deterministic.
    - `Info (string)`: Additional information. May
    be non-deterministic.
    - `GasWanted (int64)`: Amount of gas requested for transaction.
    - `GasUsed (int64)`: Amount of gas consumed by transaction.
    - `Events ([]abci.Event)`: Type & Key-Value events for indexing
    transactions (eg. by account).
    - `Codespace (string)`: Namespace for the `Code`.
- **Usage**:
    - The workhorse of the application - non-optional.
    - Execute the transaction in full.
    - `ResponseDeliverTx.Code == 0` only if the transaction is fully valid.

### EndBlock

- **Request**:
    - `Height (int64)`: Height of the block just executed.
- **Response**:
    - `ValidatorUpdates ([]ValidatorUpdate)`: Changes to validator set (set
    voting power to 0 to remove).
    - `ConsensusParamUpdates (ConsensusParams)`: Changes to
    consensus-critical time, size, and other parameters.
    - `Events ([]abci.Event)`: Type & Key-Value events for indexing
- **Usage**:
    - Signals the end of a block.
    - Called after all transactions, prior to each Commit.
    - Validator updates returned by block `H` impact blocks `H+1`, `H+2`, and
    `H+3`, but only effects changes on the validator set of `H+2`:
        - `H+1`: NextValidatorsHash
        - `H+2`: ValidatorsHash (and thus the validator set)
        - `H+3`: LastCommitInfo (ie. the last validator set)
    - Consensus params returned for block `H` apply for block `H+1`

### Commit

- **Response**:
    - `Data ([]byte)`: The Merkle root hash of the application state
    - `RetainHeight (int64)`: Blocks below this height may be removed. Defaults
    to `0` (retain all).
- **Usage**:
    - Persist the application state.
    - Return an (optional) Merkle root hash of the application state
    - `ResponseCommit.Data` is included as the `Header.AppHash` in the next block
        - it may be empty
    - Later calls to `Query` can return proofs about the application state anchored
    in this Merkle root hash
    - Note developers can return whatever they want here (could be nothing, or a
    constant string, etc.), so long as it is deterministic - it must not be a
    function of anything that did not come from the
    BeginBlock/DeliverTx/EndBlock methods.
    - Use `RetainHeight` with caution! If all nodes in the network remove historical
    blocks then this data is permanently lost, and no new nodes will be able to
    join the network and bootstrap. Historical blocks may also be required for
    other purposes, e.g. auditing, replay of non-persisted heights, light client
    verification, and so on.

### ListSnapshots

- **Response**:
    - `Snapshots ([]Snapshot)`: List of local state snapshots.
- **Usage**:
    - Used during state sync to discover available snapshots on peers.
    - See `Snapshot` data type for details.

### LoadSnapshotChunk

- **Request**:
    - `Height (uint64)`: The height of the snapshot the chunks belongs to.
    - `Format (uint32)`: The application-specific format of the snapshot the chunk belongs to.
    - `Chunk (uint32)`: The chunk index, starting from `0` for the initial chunk.
- **Response**:
    - `Chunk ([]byte)`: The binary chunk contents, in an arbitray format. Chunk messages cannot be
    larger than 16 MB _including metadata_, so 10 MB is a good starting point.
- **Usage**:
    - Used during state sync to retrieve snapshot chunks from peers.

### OfferSnapshot

- **Request**:
    - `Snapshot (Snapshot)`: The snapshot offered for restoration.
    - `AppHash ([]byte)`: The light client-verified app hash for this height, from the blockchain.
- **Response**:
    - `Result (Result)`: The result of the snapshot offer.
        - `ACCEPT`: Snapshot is accepted, start applying chunks.
        - `ABORT`: Abort snapshot restoration, and don't try any other snapshots.
        - `REJECT`: Reject this specific snapshot, try others.
        - `REJECT_FORMAT`: Reject all snapshots with this `format`, try others.
        - `REJECT_SENDERS`: Reject all snapshots from all senders of this snapshot, try others.
- **Usage**:
    - `OfferSnapshot` is called when bootstrapping a node using state sync. The application may
    accept or reject snapshots as appropriate. Upon accepting, Tendermint will retrieve and
    apply snapshot chunks via `ApplySnapshotChunk`. The application may also choose to reject a
    snapshot in the chunk response, in which case it should be prepared to accept further
    `OfferSnapshot` calls.
    - Only `AppHash` can be trusted, as it has been verified by the light client. Any other data
    can be spoofed by adversaries, so applications should employ additional verification schemes
    to avoid denial-of-service attacks. The verified `AppHash` is automatically checked against
    the restored application at the end of snapshot restoration.
    - For more information, see the `Snapshot` data type or the [state sync section](apps.md#state-sync).

### ApplySnapshotChunk

- **Request**:
    - `Index (uint32)`: The chunk index, starting from `0`. Tendermint applies chunks sequentially.
    - `Chunk ([]byte)`: The binary chunk contents, as returned by `LoadSnapshotChunk`.
    - `Sender (string)`: The P2P ID of the node who sent this chunk.
- **Response**:
    - `Result (Result)`: The result of applying this chunk.
        - `ACCEPT`: The chunk was accepted.
        - `ABORT`: Abort snapshot restoration, and don't try any other snapshots.
        - `RETRY`: Reapply this chunk, combine with `RefetchChunks` and `RejectSenders` as appropriate.
        - `RETRY_SNAPSHOT`: Restart this snapshot from `OfferSnapshot`, reusing chunks unless
      instructed otherwise.
        - `REJECT_SNAPSHOT`: Reject this snapshot, try a different one.
    - `RefetchChunks ([]uint32)`: Refetch and reapply the given chunks, regardless of `Result`. Only
    the listed chunks will be refetched, and reapplied in sequential order.
    - `RejectSenders ([]string)`: Reject the given P2P senders, regardless of `Result`. Any chunks
    already applied will not be refetched unless explicitly requested, but queued chunks from these senders will be discarded, and new chunks or other snapshots rejected.
- **Usage**:
    - The application can choose to refetch chunks and/or ban P2P peers as appropriate. Tendermint
    will not do this unless instructed by the application.
    - The application may want to verify each chunk, e.g. by attaching chunk hashes in
    `Snapshot.Metadata` and/or incrementally verifying contents against `AppHash`.
    - When all chunks have been accepted, Tendermint will make an ABCI `Info` call to verify that
    `LastBlockAppHash` and `LastBlockHeight` matches the expected values, and record the
    `AppVersion` in the node state. It then switches to fast sync or consensus and joins the
    network.
    - If Tendermint is unable to retrieve the next chunk after some time (e.g. because no suitable
    peers are available), it will reject the snapshot and try a different one via `OfferSnapshot`.
    The application should be prepared to reset and accept it or abort as appropriate.

## Data Types

### Header

- **Fields**:
    - `Version (Version)`: Version of the blockchain and the application
    - `ChainID (string)`: ID of the blockchain
    - `Height (int64)`: Height of the block in the chain
    - `Time (google.protobuf.Timestamp)`: Time of the previous block.
    For most blocks it's the weighted median of the timestamps of the valid votes in the
    block.LastCommit, except for the initial height where it's the genesis time.
    - `LastBlockID (BlockID)`: Hash of the previous (parent) block
    - `LastCommitHash ([]byte)`: Hash of the previous block's commit
    - `ValidatorsHash ([]byte)`: Hash of the validator set for this block
    - `NextValidatorsHash ([]byte)`: Hash of the validator set for the next block
    - `ConsensusHash ([]byte)`: Hash of the consensus parameters for this block
    - `AppHash ([]byte)`: Data returned by the last call to `Commit` - typically the
    Merkle root of the application state after executing the previous block's
    transactions
    - `LastResultsHash ([]byte)`: Root hash of all results from the txs from the previous block.
    - `EvidenceHash ([]byte)`: Hash of the evidence included in this block
    - `ProposerAddress ([]byte)`: Original proposer for the block
- **Usage**:
    - Provided in RequestBeginBlock
    - Provides important context about the current state of the blockchain -
    especially height and time.
    - Provides the proposer of the current block, for use in proposer-based
    reward mechanisms.
    - `LastResultsHash` is the root hash of a Merkle tree built from `ResponseDeliverTx` responses (`Log`, `Info`, `Codespace` and `Events` fields are ignored).

### Version

- **Fields**:
    - `Block (uint64)`: Protocol version of the blockchain data structures.
    - `App (uint64)`: Protocol version of the application.
- **Usage**:
    - Block version should be static in the life of a blockchain.
    - App version may be updated over time by the application.

### Validator

- **Fields**:
    - `Address ([]byte)`: Address of the validator (the first 20 bytes of SHA256(public key))
    - `Power (int64)`: Voting power of the validator
- **Usage**:
    - Validator identified by address
    - Used in RequestBeginBlock as part of VoteInfo
    - Does not include PubKey to avoid sending potentially large quantum pubkeys
    over the ABCI

### ValidatorUpdate

- **Fields**:
    - `PubKey (PubKey)`: Public key of the validator
    - `Power (int64)`: Voting power of the validator
- **Usage**:
    - Validator identified by PubKey
    - Used to tell Tendermint to update the validator set

### VoteInfo

- **Fields**:
    - `Validator (Validator)`: A validator
    - `SignedLastBlock (bool)`: Indicates whether or not the validator signed
    the last block
- **Usage**:
    - Indicates whether a validator signed the last block, allowing for rewards
    based on validator availability

### PubKey

- **Fields**:
    - `Sum (oneof PublicKey)`: This field is a Protobuf [`oneof`](https://developers.google.com/protocol-buffers/docs/proto#oneof)
- **Usage**:
    - A generic and extensible typed public key

### Evidence

- **Fields**:
    - `Type (EvidenceType)`: Type of the evidence. An enum of possible evidence's.
    - `Validator (Validator`: The offending validator
    - `Height (int64)`: Height when the offense occured
    - `Time (google.protobuf.Timestamp)`: Time of the block that was committed at the height that the offense occured
    - `TotalVotingPower (int64)`: Total voting power of the validator set at
    height `Height`

### LastCommitInfo

- **Fields**:
    - `Round (int32)`: Commit round.
    - `Votes ([]VoteInfo)`: List of validators addresses in the last validator set
    with their voting power and whether or not they signed a vote.

### ConsensusParams

- **Fields**:
    - `Block (BlockParams)`: Parameters limiting the size of a block and time between consecutive blocks.
    - `Evidence (EvidenceParams)`: Parameters limiting the validity of
    evidence of byzantine behaviour.
    - `Validator (ValidatorParams)`: Parameters limiting the types of pubkeys validators can use.
    - `Version (VersionParams)`: The ABCI application version.

### BlockParams

- **Fields**:
    - `MaxBytes (int64)`: Max size of a block, in bytes.
    - `MaxGas (int64)`: Max sum of `GasWanted` in a proposed block.
        - NOTE: blocks that violate this may be committed if there are Byzantine proposers.
      It's the application's responsibility to handle this when processing a
      block!

### EvidenceParams

- **Fields**:
    - `MaxAgeNumBlocks (int64)`: Max age of evidence, in blocks.
    - `MaxAgeDuration (time.Duration)`: Max age of evidence, in time.
    It should correspond with an app's "unbonding period" or other similar
    mechanism for handling [Nothing-At-Stake
    attacks](https://github.com/ethereum/wiki/wiki/Proof-of-Stake-FAQ#what-is-the-nothing-at-stake-problem-and-how-can-it-be-fixed).

        - Evidence older than `MaxAgeNumBlocks` && `MaxAgeDuration` is considered
      stale and ignored.
        - In Cosmos-SDK based blockchains, `MaxAgeDuration` is usually equal to the
      unbonding period. `MaxAgeNumBlocks` is calculated by dividing the unboding
      period by the average block time (e.g. 2 weeks / 6s per block = 2d8h).
    - `MaxNum (uint32)`: The maximum number of evidence that can be committed to a single block

### ValidatorParams

- **Fields**:
    - `PubKeyTypes ([]string)`: List of accepted public key types.
        - Uses same naming as `PubKey.Type`.

### VersionParams

- **Fields**:
    - `AppVersion (uint64)`: The ABCI application version.

### Proof

- **Fields**:
    - `Ops ([]ProofOp)`: List of chained Merkle proofs, of possibly different types
        - The Merkle root of one op is the value being proven in the next op.
        - The Merkle root of the final op should equal the ultimate root hash being
      verified against.

### ProofOp

- **Fields**:
    - `Type (string)`: Type of Merkle proof and how it's encoded.
    - `Key ([]byte)`: Key in the Merkle tree that this proof is for.
    - `Data ([]byte)`: Encoded Merkle proof for the key.

### Snapshot

- **Fields**:
    - `Height (uint64)`: The height at which the snapshot was taken (after commit).
    - `Format (uint32)`: An application-specific snapshot format, allowing applications to version
    their snapshot data format and make backwards-incompatible changes. Tendermint does not
    interpret this.
    - `Chunks (uint32)`: The number of chunks in the snapshot. Must be at least 1 (even if empty).
    - `Hash (bytes)`: An arbitrary snapshot hash. Must be equal only for identical snapshots across
    nodes. Tendermint does not interpret the hash, it only compares them.
    - `Metadata (bytes)`: Arbitrary application metadata, for example chunk hashes or other
    verification data.

- **Usage**:
    - Used for state sync snapshots, see [separate section](apps.md#state-sync) for details.
    - A snapshot is considered identical across nodes only if _all_ fields are equal (including
    `Metadata`). Chunks may be retrieved from all nodes that have the same snapshot.
    - When sent across the network, a snapshot message can be at most 4 MB.
