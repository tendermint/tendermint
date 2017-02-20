# Application BlockChain Interface (ABCI)

[![CircleCI](https://circleci.com/gh/tendermint/abci.svg?style=svg)](https://circleci.com/gh/tendermint/abci)

Blockchains are a system for multi-master state machine replication. 
**ABCI** is an interface that defines the boundary between the replication engine (the blockchain),
and the state machine (the application).
By using a socket protocol, we enable a consensus engine running in one process
to manage an application state running in another.

For more information on ABCI, motivations, and tutorials, please visit [our blog post](https://tendermint.com/blog/tmsp-the-tendermint-socket-protocol).

Previously, the ABCI was just referred to as TMSP.

Other implementations:
* [cpp-tmsp](https://github.com/mdyring/cpp-tmsp) by Martin Dyring-Andersen
* [js-tmsp](https://github.com/tendermint/js-tmsp)
* [jTMSP](https://github.com/jTMSP/) for Java

# Specification

The [primary specification](https://github.com/tendermint/abci/blob/master/types/types.proto) is made using Protocol Buffers.

	- The Protobuf file defining ABCI message types, and the optional GRPC interface. To build, run `make protoc`
	- See `protoc --help` and [the GRPC docs](https://www.grpc.io/docs) for examples and details of other languages.

TendermintCore runs a client, and the ABCI application runs a server. There are three Golang implementation of ABCI client and server.

1. ABCI-socket: Asynchronous, ordered message passing over Unix or TCP sockets.  Messages are serialized using Protobuf and length prefixed.
2. GRPC: Synchronous (slow) implementation using GRPC.
3. Golang in-process: If the ABCI appliation is written in Golang, it is possible to compile both TendermintCore and the application as one binary.

_TODO: merge information from https://tendermint.com/blog/tendermint-0-8-release_

## Message Types

ABCI requests/responses are simple Protobuf messages.  Check out the [schema file](https://github.com/tendermint/abci/blob/master/types/types.proto).

#### DeliverTx
  * __Arguments__:
    * `Data ([]byte)`: The request transaction bytes
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Data ([]byte)`: Result bytes, if any
    * `Log (string)`: Debug or error message
  * __Usage__:<br/>
    Append and run a transaction.  If the transaction is valid, returns CodeType.OK

#### CheckTx
  * __Arguments__:
    * `Data ([]byte)`: The request transaction bytes
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Data ([]byte)`: Result bytes, if any
    * `Log (string)`: Debug or error message
  * __Usage__:<br/>
    Validate a mempool transaction, prior to broadcasting or proposing.  This message should not mutate the main state, but application
    developers may want to keep a separate CheckTx state that gets reset upon Commit.

    CheckTx can happen interspersed with DeliverTx, but they happen on different connections - CheckTx from the mempool connection, and DeliverTx from the consensus connection.  During Commit, the mempool is locked, so you can reset the mempool state to the latest state after running all those delivertxs, and then the mempool will re-run whatever txs it has against that latest mempool state.

    Transactions are first run through CheckTx before broadcast to peers in the mempool layer.
    You can make CheckTx semi-stateful and clear the state upon `Commit` or `BeginBlock`,
    to allow for dependent sequences of transactions in the same block.

#### Commit
  * __Returns__:
    * `Data ([]byte)`: The Merkle root hash
    * `Log (string)`: Debug or error message
  * __Usage__:<br/>
    Return a Merkle root hash of the application state.

#### Query
  * __Arguments__:
    * `Data ([]byte)`: Raw query bytes.  Can be used with or in lieu of Path.
    * `Path (string)`: Path of request, like an HTTP GET path.  Can be used with or in liue of Data.
      * Apps MUST interpret '/store' as a query by key on the underlying store.  The key SHOULD be specified in the Data field.
      * Apps SHOULD allow queries over specific types like '/accounts/...' or '/votes/...'
    * `Height (uint64)`: The block height for which you want the query (default=0 returns data for the latest committed block). Note that this is the height of the block containing the application's Merkle root hash, which represents the state as it was after committing the block at Height-1
    * `Prove (bool)`: Return Merkle proof with response if possible
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Key ([]byte)`: The key of the matching data
    * `Value ([]byte)`: The value of the matching data
    * `Proof ([]byte)`: Proof for the data, if requested
    * `Height (uint64)`: The block height from which data was derived. Note that this is the height of the block containing the application's Merkle root hash, which represents the state as it was after committing the block at Height-1
    * `Log (string)`: Debug or error message
  *Please note* The current implementation of go-merkle doesn't support querying proofs from past blocks, so for the present moment, any height other than 0 will return an error (recall height=0 defaults to latest block).  Hopefully this will be improved soon(ish)

#### Info
  * __Returns__:
    * `Data (string)`: Some arbitrary information
    * `Version (Version)`: Version information
    * `LastBlockHeight (uint64)`: Latest block for which the app has called Commit
    * `LastBlockAppHash ([]byte)`: Latest result of Commit

  * __Usage__:<br/>
    Return information about the application state. Used to sync the app with Tendermint on crash/restart.

#### SetOption
  * __Arguments__:
    * `Key (string)`: Key to set
    * `Value (string)`: Value to set for key
  * __Returns__:
    * `Log (string)`: Debug or error message
  * __Usage__:<br/>
    Set application options.  E.g. Key="mode", Value="mempool" for a mempool connection, or Key="mode", Value="consensus" for a consensus connection.
    Other options are application specific.

#### InitChain
  * __Arguments__:
    * `Validators ([]Validator)`: Initial genesis validators
  * __Usage__:<br/>
    Called once upon genesis

#### BeginBlock
  * __Arguments__:
    * `Hash ([]byte)`: The block's hash.  This can be derived from the block header.
    * `Header (struct{})`: The block header
  * __Usage__:<br/>
    Signals the beginning of a new block. Called prior to any DeliverTxs. The header is expected to at least contain the Height.

#### EndBlock
  * __Arguments__:
    * `Height (uint64)`: The block height that ended
  * __Returns__:
    * `Diffs ([]Validator)`: Changed validators with new voting powers (0 to remove)
  * __Usage__:<br/>
    Signals the end of a block.  Called prior to each Commit after all transactions. Validator set is updated with the result.


# Implementations

The ABCI is a client/server interface where the replication engine (blockchain) forms the client 
and the state machine (application) forms the server.
As blocks are committed in the blockchain, they are forwarded to the application.

This repository provides two implementations of an ABCI client & server: via socket and via GRPC.

## Socket

ABCI is best implemented as a streaming protocol. 
The socket implementation provides for asynchronous, ordered message passing over unix or tcp.
Messages are serialized using Protobuf3 and length-prefixed.
Protobuf3 doesn't have an official length-prefix standard, so we use our own.  The first byte represents the length of the big-endian encoded length.

For example, if the Protobuf3 encoded ABCI message is `0xDEADBEEF` (4 bytes), the length-prefixed message is `0x0104DEADBEEF`.  If the Protobuf3 encoded ABCI message is 65535 bytes long, the length-prefixed message would be like `0x02FFFF...`.

## GRPC

GRPC is an rpc framework native to Protocol Buffers with support in many languages.
Implementing the ABCI using GRPC can allow for faster prototyping, but is expected to be much slower than 
the ordered, asynchronous socket protocol.

Note the length-prefixing used in the socket implementation does not apply for GRPC.

# Tools and Apps

The `abci-cli` tool wraps any ABCI client and can be used for probing/testing an ABCI application.
See the [tutorial](https://tendermint.com/intro/getting-started/first-tmsp) for more details.

Multiple example apps are included:
- the `counter` application, which illustrates nonce checking in txs
- the `dummy` application, which illustrates a simple key-value merkle tree
- the `dummy --persistent` application, which augments the dummy with persistence and validator set changes


# Build

To build the protobuf code:

```
make protoc
```

See `protoc --help` and [the grpc docs](https://www.grpc.io/docs) for examples and details of other languages

