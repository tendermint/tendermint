---
order: 1
title: Method and Types
---

# Methods and Types

## Connections

ABCI applications can run either within the _same_ process as the Tendermint
state-machine replication engine, or as a _separate_ process from the state-machine
replication engine. When run within the same process, Tendermint will call the ABCI
application methods directly as Go method calls.

When Tendermint and the ABCI application are run as separate processes, Tendermint
opens four connections to the application for ABCI methods. The connections each
handle a subset of the ABCI method calls. These subsets are defined as follows:

#### **Consensus** connection

* Driven by a consensus protocol and is responsible for block execution.
* Handles the `InitChain`, `BeginBlock`, `DeliverTx`, `EndBlock`, and `Commit` method
calls.

#### **Mempool** connection

* For validating new transactions, before they're shared or included in a block.
* Handles the `CheckTx` calls.

#### **Info** connection

* For initialization and for queries from the user.
* Handles the `Info` and `Query` calls.

#### **Snapshot** connection

* For serving and restoring [state sync snapshots](apps.md#state-sync).
* Handles the `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, and `ApplySnapshotChunk` calls.

Additionally, there is a `Flush` method that is called on every connection,
and an `Echo` method that is just for debugging.

More details on managing state across connections can be found in the section on
[ABCI Applications](apps.md).

## Errors

The `Query`, `CheckTx` and `DeliverTx` methods include a `Code` field in their `Response*`.
This field is meant to contain an application-specific response code.
A response code of `0` indicates no error.  Any other response code
indicates to Tendermint that an error occurred.

These methods also return a `Codespace` string to Tendermint. This field is
used to disambiguate `Code` values returned by different domains of the
application. The `Codespace` is a namespace for the `Code`.

The `Echo`, `Info`, `InitChain`, `BeginBlock`, `EndBlock`, `Commit` methods
do not return errors. An error in any of these methods represents a critical
issue that Tendermint has no reasonable way to handle. If there is an error in one
of these methods, the application must crash to ensure that the error is safely
handled by an operator.

The handling of non-zero response codes by Tendermint is described below

### CheckTx

The `CheckTx` ABCI method controls what transactions are considered for inclusion in a block.
When Tendermint receives a `ResponseCheckTx` with a non-zero `Code`, the associated
transaction will be not be added to Tendermint's mempool or it will be removed if
it is already included.

### DeliverTx

The `DeliverTx` ABCI method delivers transactions from Tendermint to the application.
When Tendermint recieves a `ResponseDeliverTx` with a non-zero `Code`, the response code is logged.
The transaction was already included in a block, so the `Code` does not influence
Tendermint consensus.

### Query

The `Query` ABCI method query queries the application for information about application state.
When Tendermint receives a `ResponseQuery` with a non-zero `Code`, this code is 
returned directly to the client that initiated the query.

## Events

The `CheckTx`, `BeginBlock`, `DeliverTx`, `EndBlock` methods include an `Events`
field in their `Response*`. Applications may respond to these ABCI methods with a set of events.
Events allow applications to associate metadata about ABCI method execution with the
transactions and blocks this metadata relates to.
Events returned via these ABCI methods do not impact Tendermint consensus in any way
and instead exist to power subscriptions and queries of Tendermint state.

An `Event` contains a `type` and a list of `EventAttributes`, which are key-value 
string pairs denoting metadata about what happened during the method's execution.
`Event` values can be used to index transactions and blocks according to what happened
during their execution. Note that the set of events returned for a block from
`BeginBlock` and `EndBlock` are merged. In case both methods return the same
key, only the value defined in `EndBlock` is used.

Each event has a `type` which is meant to categorize the event for a particular
`Response*` or `Tx`. A `Response*` or `Tx` may contain multiple events with duplicate
`type` values, where each distinct entry is meant to categorize attributes for a
particular event. Every key and value in an event's attributes must be UTF-8
encoded strings along with the event type itself.

```protobuf
message Event {
  string                  type       = 1;
  repeated EventAttribute attributes = 2;
}
```

The attributes of an `Event` consist of a `key`, a `value`, and an `index` flag. The
index flag notifies the Tendermint indexer to index the attribute. The value of
the `index` flag is non-deterministic and may vary across different nodes in the network.

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

Tendermint's security model relies on the use of "evidence". Evidence is proof of
malicious behavior by a network participant. It is the responsibility of Tendermint
to detect such malicious behavior. When malicious behavior is detected, Tendermint
will gossip evidence of the behavior to other nodes and commit the evidence to 
the chain once it is verified by all validators. This evidence will then be 
passed it on to the application through the ABCI. It is the responsibility of the
application to handle the evidence and exercise punishment.

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
or [accountability](https://github.com/tendermint/spec/blob/master/spec/light-client/accountability/)

## Determinism

ABCI applications must implement deterministic finite-state machines to be
securely replicated by the Tendermint consensus engine. This means block execution
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

* Hardware failures
    * Cosmic rays, overheating, etc.
* Node-dependent state
    * Random numbers
    * Time
* Underspecification
    * Library version changes
    * Race conditions
    * Floating point numbers
    * JSON serialization
    * Iterating through hash-tables/maps/dictionaries
* External Sources
    * Filesystem
    * Network calls (eg. some external REST API service)

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
[state sync section](../spec/p2p/messages/state-sync.md).

New nodes will discover and request snapshots from other nodes in the P2P network.
A Tendermint node that receives a request for snapshots from a peer will call
`ListSnapshots` on its application to retrieve any local state snapshots. After receiving
 snapshots from peers, the new node will offer each snapshot received from a peer
to its local application via the `OfferSnapshot` method.

Snapshots may be quite large and are thus broken into smaller "chunks" that can be
assembled into the whole snapshot. Once the application accepts a snapshot and
begins restoring it, Tendermint will fetch snapshot "chunks" from existing nodes.
The node providing "chunks" will fetch them from its local application using
the `LoadSnapshotChunk` method.

As the new node receives "chunks" it will apply them sequentially to the local
application with `ApplySnapshotChunk`. When all chunks have been applied, the application
`AppHash` is retrieved via an `Info` query. The `AppHash` is then compared to
the blockchain's `AppHash` which is verified via [light client verification](../spec/light-client/verification/README.md).

## Messages

### Echo

* **Request**:
    * `Message (string)`: A string to echo back
* **Response**:
    * `Message (string)`: The input string
* **Usage**:
    * Echo a string to test an abci client/server implementation

### Flush

* **Usage**:
    * Signals that messages queued on the client should be flushed to
    the server. It is called periodically by the client
    implementation to ensure asynchronous requests are actually
    sent, and is called immediately to make a synchronous request,
    which returns when the Flush response comes back.

### Info

* **Request**:

    | Name          | Type   | Description                              | Field Number |
    |---------------|--------|------------------------------------------|--------------|
    | version       | string | The Tendermint software semantic version | 1            |
    | block_version | uint64 | The Tendermint Block Protocol version    | 2            |
    | p2p_version   | uint64 | The Tendermint P2P Protocol version      | 3            |
    | abci_version  | string | The Tendermint ABCI semantic version     | 4            |

* **Response**:
  
    | Name                | Type   | Description                                      | Field Number |
    |---------------------|--------|--------------------------------------------------|--------------|
    | data                | string | Some arbitrary information                       | 1            |
    | version             | string | The application software semantic version        | 2            |
    | app_version         | uint64 | The application protocol version                 | 3            |
    | last_block_height   | int64  | Latest block for which the app has called Commit | 4            |
    | last_block_app_hash | bytes  | Latest result of Commit                          | 5            |

* **Usage**:
    * Return information about the application state.
    * Used to sync Tendermint with the application during a handshake
    that happens on startup.
    * The returned `app_version` will be included in the Header of every block.
    * Tendermint expects `last_block_app_hash` and `last_block_height` to
    be updated during `Commit`, ensuring that `Commit` is never
    called twice for the same block height.

> Note: Semantic version is a reference to [semantic versioning](https://semver.org/). Semantic versions in info will be displayed as X.X.x.

### InitChain

* **Request**:

    | Name             | Type                                                                                                                                 | Description                                         | Field Number |
    |------------------|--------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------|--------------|
    | time             | [google.protobuf.Timestamp](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Timestamp) | Genesis time                                        | 1            |
    | chain_id         | string                                                                                                                               | ID of the blockchain.                               | 2            |
    | consensus_params | [ConsensusParams](#consensusparams)                                                                                                  | Initial consensus-critical parameters.              | 3            |
    | validators       | repeated [ValidatorUpdate](#validatorupdate)                                                                                         | Initial genesis validators, sorted by voting power. | 4            |
    | app_state_bytes  | bytes                                                                                                                                | Serialized initial application state. JSON bytes.   | 5            |
    | initial_height   | int64                                                                                                                                | Height of the initial block (typically `1`).        | 6            |

* **Response**:

    | Name             | Type                                         | Description                                     | Field Number |
    |------------------|----------------------------------------------|-------------------------------------------------|--------------|
    | consensus_params | [ConsensusParams](#consensusparams)          | Initial consensus-critical parameters (optional | 1            |
    | validators       | repeated [ValidatorUpdate](#validatorupdate) | Initial validator set (optional).               | 2            |
    | app_hash         | bytes                                        | Initial application hash.                       | 3            |

* **Usage**:
    * Called once upon genesis.
    * If ResponseInitChain.Validators is empty, the initial validator set will be the RequestInitChain.Validators
    * If ResponseInitChain.Validators is not empty, it will be the initial
    validator set (regardless of what is in RequestInitChain.Validators).
    * This allows the app to decide if it wants to accept the initial validator
    set proposed by tendermint (ie. in the genesis file), or if it wants to use
    a different one (perhaps computed based on some application specific
    information in the genesis file).

### Query

* **Request**:
  
    | Name   | Type   | Description                                                                                                                                                                                                                                                                            | Field Number |
    |--------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
    | data   | bytes  | Raw query bytes. Can be used with or in lieu of Path.                                                                                                                                                                                                                                  | 1            |
    | path   | string | Path field of the request URI. Can be used with or in lieu of `data`. Apps MUST interpret `/store` as a query by key on the underlying store. The key SHOULD be specified in the `data` field. Apps SHOULD allow queries over specific types like `/accounts/...` or `/votes/...` | 2            |
    | height | int64  | The block height for which you want the query (default=0 returns data for the latest committed block). Note that this is the height of the block containing the application's Merkle root hash, which represents the state as it was after committing the block at Height-1            | 3            |
    | prove  | bool   | Return Merkle proof with response if possible                                                                                                                                                                                                                                          | 4            |

* **Response**:

    | Name      | Type                  | Description                                                                                                                                                                                                        | Field Number |
    |-----------|-----------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
    | code      | uint32                | Response code.                                                                                                                                                                                                     | 1            |
    | log       | string                | The output of the application's logger. **May be non-deterministic.**                                                                                                                                              | 3            |
    | info      | string                | Additional information. **May be non-deterministic.**                                                                                                                                                              | 4            |
    | index     | int64                 | The index of the key in the tree.                                                                                                                                                                                  | 5            |
    | key       | bytes                 | The key of the matching data.                                                                                                                                                                                      | 6            |
    | value     | bytes                 | The value of the matching data.                                                                                                                                                                                    | 7            |
    | proof_ops | [ProofOps](#proofops) | Serialized proof for the value data, if requested, to be verified against the `app_hash` for the given Height.                                                                                                     | 8            |
    | height    | int64                 | The block height from which data was derived. Note that this is the height of the block containing the application's Merkle root hash, which represents the state as it was after committing the block at Height-1 | 9            |
    | codespace | string                | Namespace for the `code`.                                                                                                                                                                                          | 10           |

* **Usage**:
    * Query for data from the application at current or past height.
    * Optionally return Merkle proof.
    * Merkle proof includes self-describing `type` field to support many types
    of Merkle trees and encoding formats.

### BeginBlock

* **Request**:

    | Name                 | Type                                        | Description                                                                                                       | Field Number |
    |----------------------|---------------------------------------------|-------------------------------------------------------------------------------------------------------------------|--------------|
    | hash                 | bytes                                       | The block's hash. This can be derived from the block header.                                                      | 1            |
    | header               | [Header](../core/data_structures.md#header) | The block header.                                                                                                 | 2            |
    | last_commit_info     | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round, and the list of validators and which ones signed the last block. | 3            |
    | byzantine_validators | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                            | 4            |

* **Response**:

    | Name   | Type                      | Description                         | Field Number |
    |--------|---------------------------|-------------------------------------|--------------|
    | events | repeated [Event](#events) | type & Key-Value events for indexing | 1           |

* **Usage**:
    * Signals the beginning of a new block.
    * Called prior to any `DeliverTx` method calls.
    * The header contains the height, timestamp, and more - it exactly matches the
    Tendermint block header. We may seek to generalize this in the future.
    * The `LastCommitInfo` and `ByzantineValidators` can be used to determine
    rewards and punishments for the validators.

### CheckTx

* **Request**:

    | Name | Type        | Description                                                                                                                                                                                                                                         | Field Number |
    |------|-------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
    | tx   | bytes       | The request transaction bytes                                                                                                                                                                                                                       | 1            |
    | type | CheckTxType | One of `CheckTx_New` or `CheckTx_Recheck`. `CheckTx_New` is the default and means that a full check of the tranasaction is required. `CheckTx_Recheck` types are used when the mempool is initiating a normal recheck of a transaction.             | 2            |

* **Response**:

    | Name       | Type                      | Description                                                           | Field Number |
    |------------|---------------------------|-----------------------------------------------------------------------|--------------|
    | code       | uint32                    | Response code.                                                        | 1            |
    | data       | bytes                     | Result bytes, if any.                                                 | 2            |
    | log        | string                    | The output of the application's logger. **May be non-deterministic.** | 3            |
    | info       | string                    | Additional information. **May be non-deterministic.**                 | 4            |
    | gas_wanted | int64                     | Amount of gas requested for transaction.                              | 5            |
    | gas_used   | int64                     | Amount of gas consumed by transaction.                                | 6            |
    | events     | repeated [Event](#events) | Type & Key-Value events for indexing transactions (eg. by account).   | 7            |
    | codespace  | string                    | Namespace for the `code`.                                             | 8            |
    | sender     | string                    | The transaction's sender (e.g. the signer)                            | 9            |
    | priority   | int64                     | The transaction's priority (for mempool ordering)                     | 10           |

* **Usage**:

    * Technically optional - not involved in processing blocks.
    * Guardian of the mempool: every node runs `CheckTx` before letting a
    transaction into its local mempool.
    * The transaction may come from an external user or another node
    * `CheckTx` validates the transaction against the current state of the application,
    for example, checking signatures and account balances, but does not apply any
    of the state changes described in the transaction.
    not running code in a virtual machine.
    * Transactions where `ResponseCheckTx.Code != 0` will be rejected - they will not be broadcast to
    other nodes or included in a proposal block.
    * Tendermint attributes no other value to the response code

### DeliverTx

* **Request**:

    | Name | Type  | Description                    | Field Number |
    |------|-------|--------------------------------|--------------|
    | tx   | bytes | The request transaction bytes. | 1            |

* **Response**:

    | Name       | Type                      | Description                                                           | Field Number |
    |------------|---------------------------|-----------------------------------------------------------------------|--------------|
    | code       | uint32                    | Response code.                                                        | 1            |
    | data       | bytes                     | Result bytes, if any.                                                 | 2            |
    | log        | string                    | The output of the application's logger. **May be non-deterministic.** | 3            |
    | info       | string                    | Additional information. **May be non-deterministic.**                 | 4            |
    | gas_wanted | int64                     | Amount of gas requested for transaction.                              | 5            |
    | gas_used   | int64                     | Amount of gas consumed by transaction.                                | 6            |
    | events     | repeated [Event](#events) | Type & Key-Value events for indexing transactions (eg. by account).   | 7            |
    | codespace  | string                    | Namespace for the `code`.                                             | 8            |

* **Usage**:
    * [**Required**] The core method of the application.
    * When `DeliverTx` is called, the application must execute the transaction in full before returning control to Tendermint.
    * `ResponseDeliverTx.Code == 0` only if the transaction is fully valid.

### EndBlock

* **Request**:

    | Name   | Type  | Description                        | Field Number |
    |--------|-------|------------------------------------|--------------|
    | height | int64 | Height of the block just executed. | 1            |

* **Response**:

    | Name                    | Type                                         | Description                                                     | Field Number |
    |-------------------------|----------------------------------------------|-----------------------------------------------------------------|--------------|
    | validator_updates       | repeated [ValidatorUpdate](#validatorupdate) | Changes to validator set (set voting power to 0 to remove).     | 1            |
    | consensus_param_updates | [ConsensusParams](#consensusparams)          | Changes to consensus-critical time, size, and other parameters. | 2            |
    | events                  | repeated [Event](#events)                    | Type & Key-Value events for indexing                            | 3            |

* **Usage**:
    * Signals the end of a block.
    * Called after all the transactions for the current block have been delivered, prior to the block's `Commit` message.
    * Optional `validator_updates` triggered by block `H`. These updates affect validation
      for blocks `H+1`, `H+2`, and `H+3`.
    * Heights following a validator update are affected in the following way:
        * `H+1`: `NextValidatorsHash` includes the new `validator_updates` value.
        * `H+2`: The validator set change takes effect and `ValidatorsHash` is updated.
        * `H+3`: `LastCommitInfo` is changed to include the altered validator set.
    * `consensus_param_updates` returned for block `H` apply to the consensus
      params for block `H+1`. For more information on the consensus parameters,
      see the [application spec entry on consensus parameters](../spec/abci/apps.md#consensus-parameters).

### Commit

* **Request**:

    | Name   | Type  | Description                        | Field Number |
    |--------|-------|------------------------------------|--------------|

    Commit signals the application to persist application state. It takes no parameters.
* **Response**:

    | Name          | Type  | Description                                                            | Field Number |
    |---------------|-------|------------------------------------------------------------------------|--------------|
    | data          | bytes | The Merkle root hash of the application state.                         | 2            |
    | retain_height | int64 | Blocks below this height may be removed. Defaults to `0` (retain all). | 3            |

* **Usage**:
    * Signal the application to persist the application state.
    * Return an (optional) Merkle root hash of the application state
    * `ResponseCommit.Data` is included as the `Header.AppHash` in the next block
        * it may be empty
    * Later calls to `Query` can return proofs about the application state anchored
    in this Merkle root hash
    * Note developers can return whatever they want here (could be nothing, or a
    constant string, etc.), so long as it is deterministic - it must not be a
    function of anything that did not come from the
    BeginBlock/DeliverTx/EndBlock methods.
    * Use `RetainHeight` with caution! If all nodes in the network remove historical
    blocks then this data is permanently lost, and no new nodes will be able to
    join the network and bootstrap. Historical blocks may also be required for
    other purposes, e.g. auditing, replay of non-persisted heights, light client
    verification, and so on.

### ListSnapshots

* **Request**:

    | Name   | Type  | Description                        | Field Number |
    |--------|-------|------------------------------------|--------------|

    Empty request asking the application for a list of snapshots.

* **Response**:

    | Name      | Type                           | Description                    | Field Number |
    |-----------|--------------------------------|--------------------------------|--------------|
    | snapshots | repeated [Snapshot](#snapshot) | List of local state snapshots. | 1            |

* **Usage**:
    * Used during state sync to discover available snapshots on peers.
    * See `Snapshot` data type for details.

### LoadSnapshotChunk

* **Request**:

    | Name   | Type   | Description                                                           | Field Number |
    |--------|--------|-----------------------------------------------------------------------|--------------|
    | height | uint64 | The height of the snapshot the chunks belongs to.                     | 1            |
    | format | uint32 | The application-specific format of the snapshot the chunk belongs to. | 2            |
    | chunk  | uint32 | The chunk index, starting from `0` for the initial chunk.             | 3            |

* **Response**:

    | Name  | Type  | Description                                                                                                                                           | Field Number |
    |-------|-------|-------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
    | chunk | bytes | The binary chunk contents, in an arbitray format. Chunk messages cannot be larger than 16 MB _including metadata_, so 10 MB is a good starting point. | 1            |

* **Usage**:
    * Used during state sync to retrieve snapshot chunks from peers.

### OfferSnapshot

* **Request**:

    | Name     | Type                  | Description                                                              | Field Number |
    |----------|-----------------------|--------------------------------------------------------------------------|--------------|
    | snapshot | [Snapshot](#snapshot) | The snapshot offered for restoration.                                    | 1            |
    | app_hash | bytes                 | The light client-verified app hash for this height, from the blockchain. | 2            |

* **Response**:

    | Name   | Type              | Description                       | Field Number |
    |--------|-------------------|-----------------------------------|--------------|
    | result | [Result](#result) | The result of the snapshot offer. | 1            |

#### Result

```proto
  enum Result {
    UNKNOWN       = 0;  // Unknown result, abort all snapshot restoration
    ACCEPT        = 1;  // Snapshot is accepted, start applying chunks.
    ABORT         = 2;  // Abort snapshot restoration, and don't try any other snapshots.
    REJECT        = 3;  // Reject this specific snapshot, try others.
    REJECT_FORMAT = 4;  // Reject all snapshots with this `format`, try others.
    REJECT_SENDER = 5;  // Reject all snapshots from all senders of this snapshot, try others.
  }
```

* **Usage**:
    * `OfferSnapshot` is called when bootstrapping a node using state sync. The application may
    accept or reject snapshots as appropriate. Upon accepting, Tendermint will retrieve and
    apply snapshot chunks via `ApplySnapshotChunk`. The application may also choose to reject a
    snapshot in the chunk response, in which case it should be prepared to accept further
    `OfferSnapshot` calls.
    * Only `AppHash` can be trusted, as it has been verified by the light client. Any other data
    can be spoofed by adversaries, so applications should employ additional verification schemes
    to avoid denial-of-service attacks. The verified `AppHash` is automatically checked against
    the restored application at the end of snapshot restoration.
    * For more information, see the `Snapshot` data type or the [state sync section](../spec/p2p/messages/state-sync.md).

### ApplySnapshotChunk

* **Request**:

    | Name   | Type   | Description                                                                 | Field Number |
    |--------|--------|-----------------------------------------------------------------------------|--------------|
    | index  | uint32 | The chunk index, starting from `0`. Tendermint applies chunks sequentially. | 1            |
    | chunk  | bytes  | The binary chunk contents, as returned by `LoadSnapshotChunk`.              | 2            |
    | sender | string | The P2P ID of the node who sent this chunk.                                 | 3            |

* **Response**:

    | Name           | Type                | Description                                                                                                                                                                                                                             | Field Number |
    |----------------|---------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
    | result         | Result  (see below) | The result of applying this chunk.                                                                                                                                                                                                      | 1            |
    | refetch_chunks | repeated uint32     | Refetch and reapply the given chunks, regardless of `result`. Only the listed chunks will be refetched, and reapplied in sequential order.                                                                                              | 2            |
    | reject_senders | repeated string     | Reject the given P2P senders, regardless of `Result`. Any chunks already applied will not be refetched unless explicitly requested, but queued chunks from these senders will be discarded, and new chunks or other snapshots rejected. | 3            |

```proto
  enum Result {
    UNKNOWN         = 0;  // Unknown result, abort all snapshot restoration
    ACCEPT          = 1;  // The chunk was accepted.
    ABORT           = 2;  // Abort snapshot restoration, and don't try any other snapshots.
    RETRY           = 3;  // Reapply this chunk, combine with `RefetchChunks` and `RejectSenders` as appropriate.
    RETRY_SNAPSHOT  = 4;  // Restart this snapshot from `OfferSnapshot`, reusing chunks unless instructed otherwise.
    REJECT_SNAPSHOT = 5;  // Reject this snapshot, try a different one.
  }
```

* **Usage**:
    * The application can choose to refetch chunks and/or ban P2P peers as appropriate. Tendermint
    will not do this unless instructed by the application.
    * The application may want to verify each chunk, e.g. by attaching chunk hashes in
    `Snapshot.Metadata` and/or incrementally verifying contents against `AppHash`.
    * When all chunks have been accepted, Tendermint will make an ABCI `Info` call to verify that
    `LastBlockAppHash` and `LastBlockHeight` matches the expected values, and record the
    `AppVersion` in the node state. It then switches to fast sync or consensus and joins the
    network.
    * If Tendermint is unable to retrieve the next chunk after some time (e.g. because no suitable
    peers are available), it will reject the snapshot and try a different one via `OfferSnapshot`.
    The application should be prepared to reset and accept it or abort as appropriate.

## Data Types

Most of the data structures used in ABCI are shared [common data structures](../spec/core/data_structures.md). In certain cases, ABCI uses different data structures which are documented here:

### Validator

* **Fields**:

    | Name    | Type  | Description                                                         | Field Number |
    |---------|-------|---------------------------------------------------------------------|--------------|
    | address | bytes | [Address](../core/data_structures.md#address) of validator          | 1            |
    | power   | int64 | Voting power of the validator                                       | 3            |

* **Usage**:
    * Validator identified by address
    * Used in RequestBeginBlock as part of VoteInfo
    * Does not include PubKey to avoid sending potentially large quantum pubkeys
    over the ABCI

### ValidatorUpdate

* **Fields**:

    | Name    | Type                                             | Description                   | Field Number |
    |---------|--------------------------------------------------|-------------------------------|--------------|
    | pub_key | [Public Key](../core/data_structures.md#pub_key) | Public key of the validator   | 1            |
    | power   | int64                                            | Voting power of the validator | 2            |

* **Usage**:
    * Validator identified by PubKey
    * Used to tell Tendermint to update the validator set

### VoteInfo

* **Fields**:

    | Name              | Type                    | Description                                                  | Field Number |
    |-------------------|-------------------------|--------------------------------------------------------------|--------------|
    | validator         | [Validator](#validator) | A validator                                                  | 1            |
    | signed_last_block | bool                    | Indicates whether or not the validator signed the last block | 2            |

* **Usage**:
    * Indicates whether a validator signed the last block, allowing for rewards
    based on validator availability

### Evidence

* **Fields**:

    | Name               | Type                                                                                                                                 | Description                                                                  | Field Number |
    |--------------------|--------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------|--------------|
    | type               | [EvidenceType](#evidencetype)                                                                                                        | Type of the evidence. An enum of possible evidence's.                        | 1            |
    | validator          | [Validator](#validator)                                                                                                              | The offending validator                                                      | 2            |
    | height             | int64                                                                                                                                | Height when the offense occurred                                             | 3            |
    | time               | [google.protobuf.Timestamp](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Timestamp) | Time of the block that was committed at the height that the offense occurred | 4            |
    | total_voting_power | int64                                                                                                                                | Total voting power of the validator set at height `Height`                   | 5            |

#### EvidenceType

* **Fields**

    EvidenceType is an enum with the listed fields:

    | Name                | Field Number |
    |---------------------|--------------|
    | UNKNOWN             | 0            |
    | DUPLICATE_VOTE      | 1            |
    | LIGHT_CLIENT_ATTACK | 2            |

### LastCommitInfo

* **Fields**:

    | Name  | Type                           | Description                                                                                                           | Field Number |
    |-------|--------------------------------|-----------------------------------------------------------------------------------------------------------------------|--------------|
    | round | int32                          | Commit round. Reflects the total amount of rounds it took to come to consensus for the current block.                 | 1            |
    | votes | repeated [VoteInfo](#voteinfo) | List of validators addresses in the last validator set with their voting power and whether or not they signed a vote. | 2            |

### ConsensusParams

* **Fields**:

    | Name      | Type                                                          | Description                                                                  | Field Number |
    |-----------|---------------------------------------------------------------|------------------------------------------------------------------------------|--------------|
    | block     | [BlockParams](../core/data_structures.md#blockparams)                                   | Parameters limiting the size of a block and time between consecutive blocks. | 1            |
    | evidence  | [EvidenceParams](../core/data_structures.md#evidenceparams)   | Parameters limiting the validity of evidence of byzantine behavior.         | 2            |
    | validator | [ValidatorParams](../core/data_structures.md#validatorparams) | Parameters limiting the types of public keys validators can use.             | 3            |
    | version   | [VersionsParams](../core/data_structures.md#versionparams)       | The ABCI application version.                                                | 4            |

### ProofOps

* **Fields**:

    | Name | Type                         | Description                                                                                                                                                                                                                  | Field Number |
    |------|------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
    | ops  | repeated [ProofOp](#proofop) | List of chained Merkle proofs, of possibly different types. The Merkle root of one op is the value being proven in the next op. The Merkle root of the final op should equal the ultimate root hash being verified against.. | 1            |

### ProofOp

* **Fields**:

    | Name | Type   | Description                                    | Field Number |
    |------|--------|------------------------------------------------|--------------|
    | type | string | Type of Merkle proof and how it's encoded.     | 1            |
    | key  | bytes  | Key in the Merkle tree that this proof is for. | 2            |
    | data | bytes  | Encoded Merkle proof for the key.              | 3            |

### Snapshot

* **Fields**:

    | Name     | Type   | Description                                                                                                                                                                       | Field Number |
    |----------|--------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
    | height   | uint64 | The height at which the snapshot was taken (after commit).                                                                                                                        | 1            |
    | format   | uint32 | An application-specific snapshot format, allowing applications to version their snapshot data format and make backwards-incompatible changes. Tendermint does not interpret this. | 2            |
    | chunks   | uint32 | The number of chunks in the snapshot. Must be at least 1 (even if empty).                                                                                                         | 3            |
    | hash     | bytes  | TAn arbitrary snapshot hash. Must be equal only for identical snapshots across nodes. Tendermint does not interpret the hash, it only compares them.                              | 3            |
    | metadata | bytes  | Arbitrary application metadata, for example chunk hashes or other verification data.                                                                                              | 3            |

* **Usage**:
    * Used for state sync snapshots, see the [state sync section](../spec/p2p/messages/state-sync.md) for details.
    * A snapshot is considered identical across nodes only if _all_ fields are equal (including
    `Metadata`). Chunks may be retrieved from all nodes that have the same snapshot.
    * When sent across the network, a snapshot message can be at most 4 MB.
