# ABCI Specification

## Message Types

ABCI requests/responses are defined as simple Protobuf messages in [this
schema file](https://github.com/tendermint/tendermint/blob/master/abci/types/types.proto).
TendermintCore sends the requests, and the ABCI application sends the
responses. Here, we provide an overview of the messages types and how
they are used by Tendermint. Then we describe each request-response pair
as a function with arguments and return values, and add some notes on
usage.

Some messages (`Echo, Info, InitChain, BeginBlock, EndBlock, Commit`),
don't return errors because an error would indicate a critical failure
in the application and there's nothing Tendermint can do. The problem
should be addressed and both Tendermint and the application restarted.
All other messages (`SetOption, Query, CheckTx, DeliverTx`) return an
application-specific response `Code uint32`, where only `0` is reserved
for `OK`.

Some messages (`SetOption, Query, CheckTx, DeliverTx`) return
non-deterministic data in the form of `Info` and `Log`. The `Log` is
intended for the literal output from the application's logger, while the
`Info` is any additional info that should be returned.

The first time a new blockchain is started, Tendermint calls
`InitChain`. From then on, the Block Execution Sequence that causes the
committed state to be updated is as follows:

`BeginBlock, [DeliverTx], EndBlock, Commit`

where one `DeliverTx` is called for each transaction in the block.
Cryptographic commitments to the results of DeliverTx, EndBlock, and
Commit are included in the header of the next block.

Tendermint opens three connections to the application to handle the
different message types:

- `Consensus Connection - InitChain, BeginBlock, DeliverTx, EndBlock, Commit`
- `Mempool Connection - CheckTx`
- `Info Connection - Info, SetOption, Query`

The `Flush` message is used on every connection, and the `Echo` message
is only used for debugging.

Note that messages may be sent concurrently across all connections -a
typical application will thus maintain a distinct state for each
connection. They may be referred to as the `DeliverTx state`, the
`CheckTx state`, and the `Commit state` respectively.

See below for more details on the message types and how they are used.

## Request/Response Messages

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
  - `Version (string)`: The Tendermint version
- **Response**:
  - `Data (string)`: Some arbitrary information
  - `Version (Version)`: Version information
  - `LastBlockHeight (int64)`: Latest block for which the app has
    called Commit
  - `LastBlockAppHash ([]byte)`: Latest result of Commit
- **Usage**:
  - Return information about the application state.
  - Used to sync Tendermint with the application during a handshake
    that happens on startup.
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
  - `Validators ([]Validator)`: Initial genesis validators.
  - `AppStateBytes ([]byte)`: Serialized initial application state. Amino-encoded JSON bytes.
- **Response**:
  - `ConsensusParams (ConsensusParams)`: Initial
    consensus-critical parameters.
  - `Validators ([]Validator)`: Initial validator set.
- **Usage**:
  - Called once upon genesis.

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
  - `Proof ([]byte)`: Proof for the data, if requested.
  - `Height (int64)`: The block height from which data was derived.
    Note that this is the height of the block containing the
    application's Merkle root hash, which represents the state as it
    was after committing the block at Height-1
- **Usage**:
  - Query for data from the application at current or past height.
  - Optionally return Merkle proof.

### BeginBlock

- **Request**:
  - `Hash ([]byte)`: The block's hash. This can be derived from the
    block header.
  - `Header (struct{})`: The block header.
  - `LastCommitInfo (LastCommitInfo)`: Info about the last commit.
  - `ByzantineValidators ([]Evidence)`: List of evidence of
    validators that acted maliciously
- **Response**:
  - `Tags ([]cmn.KVPair)`: Key-Value tags for filtering and indexing
- **Usage**:
  - Signals the beginning of a new block. Called prior to
    any DeliverTxs.
  - The header is expected to at least contain the Height.
  - The `Validators` and `ByzantineValidators` can be used to
    determine rewards and punishments for the validators.

### CheckTx

- **Request**:
  - `Tx ([]byte)`: The request transaction bytes
- **Response**:
  - `Code (uint32)`: Response code
  - `Data ([]byte)`: Result bytes, if any.
  - `Log (string)`: The output of the application's logger. May
    be non-deterministic.
  - `Info (string)`: Additional information. May
    be non-deterministic.
  - `GasWanted (int64)`: Amount of gas request for transaction.
  - `GasUsed (int64)`: Amount of gas consumed by transaction.
  - `Tags ([]cmn.KVPair)`: Key-Value tags for filtering and indexing
    transactions (eg. by account).
- **Usage**: Validate a mempool transaction, prior to broadcasting
  or proposing. CheckTx should perform stateful but light-weight
  checks of the validity of the transaction (like checking signatures
  and account balances), but need not execute in full (like running a
  smart contract).

  Tendermint runs CheckTx and DeliverTx concurrently with eachother,
  though on distinct ABCI connections - the mempool connection and the
  consensus connection, respectively.

  The application should maintain a separate state to support CheckTx.
  This state can be reset to the latest committed state during
  `Commit`. Before calling Commit, Tendermint will lock and flush the mempool,
  ensuring that all existing CheckTx are responded to and no new ones can
  begin. After `Commit`, the mempool will rerun
  CheckTx for all remaining transactions, throwing out any that are no longer valid.
  Then the mempool will unlock and start sending CheckTx again.

  Keys and values in Tags must be UTF-8 encoded strings (e.g.
  "account.owner": "Bob", "balance": "100.0", "date": "2018-01-02")

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
- **Usage**:
  - Deliver a transaction to be executed in full by the application.
    If the transaction is valid, returns CodeType.OK.
  - Keys and values in Tags must be UTF-8 encoded strings (e.g.
    "account.owner": "Bob", "balance": "100.0",
    "time": "2018-01-02T12:30:00Z")

### EndBlock

- **Request**:
  - `Height (int64)`: Height of the block just executed.
- **Response**:
  - `ValidatorUpdates ([]Validator)`: Changes to validator set (set
    voting power to 0 to remove).
  - `ConsensusParamUpdates (ConsensusParams)`: Changes to
    consensus-critical time, size, and other parameters.
  - `Tags ([]cmn.KVPair)`: Key-Value tags for filtering and indexing
- **Usage**:
  - Signals the end of a block.
  - Called prior to each Commit, after all transactions.
  - Validator set and consensus params are updated with the result.
  - Validator pubkeys are expected to be go-wire encoded.

### Commit

- **Response**:
  - `Data ([]byte)`: The Merkle root hash
- **Usage**:
  - Persist the application state.
  - Return a Merkle root hash of the application state.
  - It's critical that all application instances return the
    same hash. If not, they will not be able to agree on the next
    block, because the hash is included in the next block!

## Data Messages

### Header

- **Fields**:
  - `ChainID (string)`: ID of the blockchain
  - `Height (int64)`: Height of the block in the chain
  - `Time (google.protobuf.Timestamp)`: Time of the block. It is the proposer's
    local time when block was created.
  - `NumTxs (int32)`: Number of transactions in the block
  - `TotalTxs (int64)`: Total number of transactions in the blockchain until
    now
  - `LastBlockHash ([]byte)`: Hash of the previous (parent) block
  - `ValidatorsHash ([]byte)`: Hash of the validator set for this block
  - `AppHash ([]byte)`: Data returned by the last call to `Commit` - typically the
    Merkle root of the application state after executing the previous block's
    transactions
  - `Proposer (Validator)`: Original proposer for the block
- **Usage**:
  - Provided in RequestBeginBlock
  - Provides important context about the current state of the blockchain -
    especially height and time.
  - Provides the proposer of the current block, for use in proposer-based
    reward mechanisms.

### Validator

- **Fields**:
  - `Address ([]byte)`: Address of the validator (hash of the public key)
  - `PubKey (PubKey)`: Public key of the validator
  - `Power (int64)`: Voting power of the validator
- **Usage**:
  - Provides all identifying information about the validator

### SigningValidator

- **Fields**:
  - `Validator (Validator)`: A validator
  - `SignedLastBlock (bool)`: Indicated whether or not the validator signed
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
  - `CommitRound (int32)`: Commit round.
  - `Validators ([]SigningValidator)`: List of validators in the current
    validator set and whether or not they signed a vote.
