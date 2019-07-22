# Methods and Types

## Overview

The ABCI message types are defined in a [protobuf
file](https://github.com/tendermint/tendermint/blob/master/abci/types/types.proto).

ABCI methods are split across 3 separate ABCI _connections_:

- `Consensus Connection`: `InitChain, BeginBlock, DeliverTx, EndBlock, Commit`
- `Mempool Connection`: `CheckTx`
- `Info Connection`: `Info, SetOption, Query`

The `Consensus Connection` is driven by a consensus protocol and is responsible
for block execution.
The `Mempool Connection` is for validating new transactions, before they're
shared or included in a block.
The `Info Connection` is for initialization and for queries from the user.

Additionally, there is a `Flush` method that is called on every connection,
and an `Echo` method that is just for debugging.

More details on managing state across connections can be found in the section on
[ABCI Applications](apps.md).

## Errors

Some methods (`Echo, Info, InitChain, BeginBlock, EndBlock, Commit`),
don't return errors because an error would indicate a critical failure
in the application and there's nothing Tendermint can do. The problem
should be addressed and both Tendermint and the application restarted.

All other methods (`SetOption, Query, CheckTx, DeliverTx`) return an
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
encoded strings along with the even type itself.

Example:

```go
 abci.ResponseDeliverTx{
 	// ...
	Events: []abci.Event{
		{
			Type: "validator.provisions",
			Attributes: cmn.KVPairs{
				cmn.KVPair{Key: []byte("address"), Value: []byte("...")},
				cmn.KVPair{Key: []byte("amount"), Value: []byte("...")},
				cmn.KVPair{Key: []byte("balance"), Value: []byte("...")},
			},
		},
		{
			Type: "validator.provisions",
			Attributes: cmn.KVPairs{
				cmn.KVPair{Key: []byte("address"), Value: []byte("...")},
				cmn.KVPair{Key: []byte("amount"), Value: []byte("...")},
				cmn.KVPair{Key: []byte("balance"), Value: []byte("...")},
			},
		},
		{
			Type: "validator.slashed",
			Attributes: cmn.KVPairs{
				cmn.KVPair{Key: []byte("address"), Value: []byte("...")},
				cmn.KVPair{Key: []byte("amount"), Value: []byte("...")},
				cmn.KVPair{Key: []byte("reason"), Value: []byte("...")},
			},
		},
		// ...
	},
}
```

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

Note that some methods (`SetOption, Query, CheckTx, DeliverTx`) return
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

### SetOption

- **Request**:
  - `Key (string)`: Key to set
  - `Value (string)`: Value to set for key
- **Response**:
  - `Code (uint32)`: Response code
  - `Log (string)`: The output of the application's logger. May
    be non-deterministic.
  - `Info (string)`: Additional information. May
    be non-deterministic.
- **Usage**:
  - Set non-consensus critical application specific options.
  - e.g. Key="min-fee", Value="100fermion" could set the minimum fee
    required for CheckTx (but not DeliverTx - that would be
    consensus critical).

### InitChain

- **Request**:
  - `Time (google.protobuf.Timestamp)`: Genesis time.
  - `ChainID (string)`: ID of the blockchain.
  - `ConsensusParams (ConsensusParams)`: Initial consensus-critical parameters.
  - `Validators ([]ValidatorUpdate)`: Initial genesis validators.
  - `AppStateBytes ([]byte)`: Serialized initial application state. Amino-encoded JSON bytes.
- **Response**:
  - `ConsensusParams (ConsensusParams)`: Initial
    consensus-critical parameters.
  - `Validators ([]ValidatorUpdate)`: Initial validator set (if non empty).
- **Usage**:
  - Called once upon genesis.
  - If ResponseInitChain.Validators is empty, the initial validator set will be the RequestInitChain.Validators
  - If ResponseInitChain.Validators is not empty, the initial validator set will be the
    ResponseInitChain.Validators (regardless of what is in RequestInitChain.Validators).
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
  - `Tags ([]cmn.KVPair)`: Key-Value tags for filtering and indexing
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
  - `Tags ([]cmn.KVPair)`: Key-Value tags for filtering and indexing
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
  - `Tags ([]cmn.KVPair)`: Key-Value tags for filtering and indexing
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
  - `Tags ([]cmn.KVPair)`: Key-Value tags for filtering and indexing
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

## Data Types

### Header

- **Fields**:
  - `Version (Version)`: Version of the blockchain and the application
  - `ChainID (string)`: ID of the blockchain
  - `Height (int64)`: Height of the block in the chain
  - `Time (google.protobuf.Timestamp)`: Time of the previous block.
    For heights > 1, it's the weighted median of the timestamps of the valid
    votes in the block.LastCommit.
    For height == 1, it's genesis time.
  - `NumTxs (int32)`: Number of transactions in the block
  - `TotalTxs (int64)`: Total number of transactions in the blockchain until
    now
  - `LastBlockID (BlockID)`: Hash of the previous (parent) block
  - `LastCommitHash ([]byte)`: Hash of the previous block's commit
  - `ValidatorsHash ([]byte)`: Hash of the validator set for this block
  - `NextValidatorsHash ([]byte)`: Hash of the validator set for the next block
  - `ConsensusHash ([]byte)`: Hash of the consensus parameters for this block
  - `AppHash ([]byte)`: Data returned by the last call to `Commit` - typically the
    Merkle root of the application state after executing the previous block's
    transactions
  - `LastResultsHash ([]byte)`: Hash of the ABCI results returned by the last block
  - `EvidenceHash ([]byte)`: Hash of the evidence included in this block
  - `ProposerAddress ([]byte)`: Original proposer for the block
- **Usage**:
  - Provided in RequestBeginBlock
  - Provides important context about the current state of the blockchain -
    especially height and time.
  - Provides the proposer of the current block, for use in proposer-based
    reward mechanisms.

### Version

- **Fields**:
  - `Block (uint64)`: Protocol version of the blockchain data structures.
  - `App (uint64)`: Protocol version of the application.
- **Usage**:
  - Block version should be static in the life of a blockchain.
  - App version may be updated over time by the application.

### Validator

- **Fields**:
  - `Address ([]byte)`: Address of the validator (hash of the public key)
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
  - `Type (string)`: Type of the public key. A simple string like `"ed25519"`.
    In the future, may indicate a serialization algorithm to parse the `Data`,
    for instance `"amino"`.
  - `Data ([]byte)`: Public key data. For a simple public key, it's just the
    raw bytes. If the `Type` indicates an encoding algorithm, this is the
    encoded public key.
- **Usage**:
  - A generic and extensible typed public key

### Evidence

- **Fields**:
  - `Type (string)`: Type of the evidence. A hierarchical path like
    "duplicate/vote".
  - `Validator (Validator`: The offending validator
  - `Height (int64)`: Height when the offense was committed
  - `Time (google.protobuf.Timestamp)`: Time of the block at height `Height`.
    It is the proposer's local time when block was created.
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
  - `Validator (ValidatorParams)`: Parameters limitng the types of pubkeys validators can use.

### BlockParams

- **Fields**:
  - `MaxBytes (int64)`: Max size of a block, in bytes.
  - `MaxGas (int64)`: Max sum of `GasWanted` in a proposed block.
    - NOTE: blocks that violate this may be committed if there are Byzantine proposers.
      It's the application's responsibility to handle this when processing a
      block!

### EvidenceParams

- **Fields**:
  - `MaxAge (int64)`: Max age of evidence, in blocks. Evidence older than this
    is considered stale and ignored.
    - This should correspond with an app's "unbonding period" or other
      similar mechanism for handling Nothing-At-Stake attacks.
    - NOTE: this should change to time (instead of blocks)!

### ValidatorParams

- **Fields**:
  - `PubKeyTypes ([]string)`: List of accepted pubkey types. Uses same
    naming as `PubKey.Type`.

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
