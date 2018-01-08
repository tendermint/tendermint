# Application BlockChain Interface (ABCI)

[![CircleCI](https://circleci.com/gh/tendermint/abci.svg?style=svg)](https://circleci.com/gh/tendermint/abci)

Blockchains are systems for multi-master state machine replication.
**ABCI** is an interface that defines the boundary between the replication engine (the blockchain),
and the state machine (the application).
By using a socket protocol, we enable a consensus engine running in one process
to manage an application state running in another.

For background information on ABCI, motivations, and tendermint, please visit [the documentation](http://tendermint.readthedocs.io/en/master/).
The two guides to focus on are the `Application Development Guide` and `Using ABCI-CLI`.

Previously, the ABCI was referred to as TMSP.

The community has provided a number of addtional implementations, see the `Tendermint Ecosystem` in [the documentation](http://tendermint.readthedocs.io/en/master/).

## Implementation

We provide three implementations of the ABCI in Go:

- Golang in-process
- ABCI-socket
- GRPC

Note the GRPC version is maintained primarily to simplify onboarding and prototyping and is not receiving the same
attention to security and performance as the others.

### In Process

The simplest implementation just uses function calls within Go.
This means ABCI applications written in Golang can be compiled with TendermintCore and run as a single binary.

### Socket (TSP)

ABCI is best implemented as a streaming protocol.
The socket implementation provides for asynchronous, ordered message passing over unix or tcp.
Messages are serialized using Protobuf3 and length-prefixed.
Protobuf3 doesn't have an official length-prefix standard, so we use our own.  The first byte represents the length of the big-endian encoded length.

For example, if the Protobuf3 encoded ABCI message is `0xDEADBEEF` (4 bytes), the length-prefixed message is `0x0104DEADBEEF`.  If the Protobuf3 encoded ABCI message is 65535 bytes long, the length-prefixed message would be like `0x02FFFF...`.

### GRPC

GRPC is an rpc framework native to Protocol Buffers with support in many languages.
Implementing the ABCI using GRPC can allow for faster prototyping, but is expected to be much slower than
the ordered, asynchronous socket protocol. The implementation has also not received as much testing or review.

Note the length-prefixing used in the socket implementation does not apply for GRPC.

## Tools

The `abci-cli` tool wraps an ABCI client and can be used for probing/testing an ABCI server.
For instance, `abci-cli test` will run a test sequence against a listening server running the Counter application (see below).
It can also be used to run some example applications.
See [the documentation](http://tendermint.readthedocs.io/en/master/) for more details.

### Example Apps

Multiple example apps are included:
- the `abci-cli counter` application, which illustrates nonce checking in txs
- the `abci-cli dummy` application, which illustrates a simple key-value Merkle tree
- the `abci-cli dummy --persistent` application, which augments the dummy with persistence and validator set changes

### Install

```
go get github.com/tendermint/abci
cd $GOPATH/src/github.com/tendermint/abci
make get_vendor_deps
make install
```

## Specification

The [primary specification](https://github.com/tendermint/abci/blob/master/types/types.proto) is made using Protocol Buffers.
To build it, run

```
make protoc
```

See `protoc --help` and [the Protocol Buffers site](https://developers.google.com/protocol-buffers/) for details on compiling for other languages.
Note we also include a [GRPC](http://www.grpc.io/docs) service definition.

For the specification as an interface in Go, see the [types/application.go file](https://github.com/tendermint/abci/blob/master/types/application.go).

### Message Types

ABCI requests/responses are defined as simple Protobuf messages in [this schema file](https://github.com/tendermint/abci/blob/master/types/types.proto).
TendermintCore sends the requests, and the ABCI application sends the responses.
Here, we describe the requests and responses as function arguments and return values, and make some notes about usage:

#### Echo
  * __Arguments__:
    * `Message (string)`: A string to echo back
  * __Returns__:
    * `Message (string)`: The input string
  * __Usage__:<br/>
    * Echo a string to test an abci client/server implementation

#### Flush
  * __Usage__:<br/>
    * Signals that messages queued on the client should be flushed to the server. It is called periodically by the client implementation to ensure asynchronous requests are actually sent, and is called immediately to make a synchronous request, which returns when the Flush response comes back.

#### Info
  * __Returns__:
    * `Data (string)`: Some arbitrary information
    * `Version (Version)`: Version information
    * `LastBlockHeight (int64)`: Latest block for which the app has called Commit
    * `LastBlockAppHash ([]byte)`: Latest result of Commit

  * __Usage__:<br/>
    Return information about the application state. Used to sync the app with Tendermint on crash/restart.

#### SetOption
  * __Arguments__:
    * `Key (string)`: Key to set
    * `Value (string)`: Value to set for key
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Log (string)`: Debug or error message
  * __Usage__:<br/>
    Set application options.  E.g. Key="mode", Value="mempool" for a mempool connection, or Key="mode", Value="consensus" for a consensus connection.
    Other options are application specific.

#### InitChain
  * __Arguments__:
    * `Validators ([]Validator)`: Initial genesis validators
  * __Usage__:<br/>
    Called once upon genesis

#### Query
  * __Arguments__:
    * `Data ([]byte)`: Raw query bytes.  Can be used with or in lieu of Path.
    * `Path (string)`: Path of request, like an HTTP GET path.  Can be used with or in liue of Data.
      * Apps MUST interpret '/store' as a query by key on the underlying store.  The key SHOULD be specified in the Data field.
      * Apps SHOULD allow queries over specific types like '/accounts/...' or '/votes/...'
    * `Height (int64)`: The block height for which you want the query (default=0 returns data for the latest committed block). Note that this is the height of the block containing the application's Merkle root hash, which represents the state as it was after committing the block at Height-1
    * `Prove (bool)`: Return Merkle proof with response if possible
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Key ([]byte)`: The key of the matching data
    * `Value ([]byte)`: The value of the matching data
    * `Proof ([]byte)`: Proof for the data, if requested
    * `Height (int64)`: The block height from which data was derived. Note that this is the height of the block containing the application's Merkle root hash, which represents the state as it was after committing the block at Height-1
    * `Log (string)`: Debug or error message

#### BeginBlock
  * __Arguments__:
    * `Hash ([]byte)`: The block's hash.  This can be derived from the block header.
    * `Header (struct{})`: The block header
    * `AbsentValidators ([]int32)`: List of indices of validators not included in the LastCommit
    * `ByzantineValidators ([]Evidence)`: List of evidence of validators that acted maliciously
  * __Usage__:<br/>
    Signals the beginning of a new block. Called prior to any DeliverTxs. The header is expected to at least contain the Height. The `AbsentValidators` and `ByzantineValidators` can be used to determine rewards and punishments for the validators.

#### CheckTx
  * __Arguments__:
    * `Data ([]byte)`: The request transaction bytes
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Data ([]byte)`: Result bytes, if any
    * `Log (string)`: Debug or error message
    * `Gas (int64)`: Amount of gas consumed by transaction
    * `Fee (int64)`: Fee paid by transaction
  * __Usage__:<br/>
    Validate a mempool transaction, prior to broadcasting or proposing.  This message should not mutate the main state, but application
    developers may want to keep a separate CheckTx state that gets reset upon Commit.

    CheckTx can happen interspersed with DeliverTx, but they happen on different ABCI connections - CheckTx from the mempool connection, and DeliverTx from the consensus connection.  During Commit, the mempool is locked, so you can reset the mempool state to the latest state after running all those DeliverTxs, and then the mempool will re-run whatever txs it has against that latest mempool state.

    Transactions are first run through CheckTx before broadcast to peers in the mempool layer.
    You can make CheckTx semi-stateful and clear the state upon `Commit` or `BeginBlock`,
    to allow for dependent sequences of transactions in the same block.

#### DeliverTx
  * __Arguments__:
    * `Data ([]byte)`: The request transaction bytes
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Data ([]byte)`: Result bytes, if any
    * `Log (string)`: Debug or error message
    * `Tags ([]*KVPair)`: Optional tags for indexing
  * __Usage__:<br/>
    Append and run a transaction.  If the transaction is valid, returns CodeType.OK

#### EndBlock
  * __Arguments__:
    * `Height (int64)`: The block height that ended
  * __Returns__:
    * `ValidatorUpdates ([]Validator)`: Changes to validator set (set voting power to 0 to remove)
    * `ConsensusParamUpdates (ConsensusParams)`: Changes to consensus-critical time/size parameters
  * __Usage__:<br/>
    Signals the end of a block.  Called prior to each Commit after all transactions. Validator set is updated with the result.

#### Commit
  * __Returns__:
    * `Data ([]byte)`: The Merkle root hash
    * `Log (string)`: Debug or error message
  * __Usage__:<br/>
    Return a Merkle root hash of the application state.

