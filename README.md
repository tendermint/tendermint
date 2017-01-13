# Tendermint Socket Protocol (ABCI)

[![CircleCI](https://circleci.com/gh/tendermint/abci.svg?style=svg)](https://circleci.com/gh/tendermint/abci)

Blockchains are a system for creating shared multi-master application state. 
**ABCI** is a socket protocol enabling a blockchain consensus engine, running in one process,
to manage a blockchain application state, running in another.

For more information on ABCI, motivations, and tutorials, please visit [our blog post](https://tendermint.com/blog/abci-the-tendermint-socket-protocol).

Other implementations:
* [cpp-abci](https://github.com/mdyring/cpp-abci) by Martin Dyring-Andersen
* [js-abci](https://github.com/tendermint/js-abci)
* [jABCI](https://github.com/jABCI/) for Java

## Contents

This repository holds a number of important pieces:

- `types/types.proto`
	- the protobuf file defining ABCI message types, and the optional grpc interface. 
        - to build, run `make protoc`
	- see `protoc --help` and [the grpc docs](https://www.grpc.io/docs) for examples and details of other languages

- golang implementation of ABCI client and server
	- two implementations:
		- asynchronous, ordered message passing over unix or tcp; 
			- messages are serialized using protobuf and length prefixed
		- grpc 
	- TendermintCore runs a client, and the application runs a server

- `cmd/abci-cli`
	- command line tool wrapping the client for probing/testing a ABCI application
	- use `abci-cli --version` to get the ABCI version

- examples:
	- the `cmd/counter` application, which illustrates nonce checking in txs
	- the `cmd/dummy` application, which illustrates a simple key-value merkle tree


## Message format

Since this is a streaming protocol, all messages are encoded with a length-prefix followed by the message encoded in Protobuf3.  Protobuf3 doesn't have an official length-prefix standard, so we use our own.  The first byte represents the length of the big-endian encoded length.

For example, if the Protobuf3 encoded ABCI message is `0xDEADBEEF` (4 bytes), the length-prefixed message is `0x0104DEADBEEF`.  If the Protobuf3 encoded ABCI message is 65535 bytes long, the length-prefixed message would be like `0x02FFFF...`.

Note this prefixing does not apply for grpc.

## Message types

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

    CheckTx can happen interspersed with DeliverTx, but they happen on different connections - CheckTx from the mempool connection, and DeliverTx from the consensus connection.  During Commit, the mempool is locked, so you can reset the mempool state to the latest state after running all those delivertxs, and then the mempool will re run whatever txs it has against that latest mempool stte

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
    * `Data ([]byte)`: The query request bytes
  * __Returns__:
    * `Code (uint32)`: Response code
    * `Data ([]byte)`: The query response bytes
    * `Log (string)`: Debug or error message

#### Flush
  * __Usage__:<br/>
    Flush the response queue.  Applications that implement `types.Application` need not implement this message -- it's handled by the project.

#### Info
  * __Returns__:
    * `Data ([]byte)`: The info bytes
  * __Usage__:<br/>
    Return information about the application state.  Application specific.

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
    * `Height (uint64)`: The block height that is starting
  * __Usage__:<br/>
    Signals the beginning of a new block. Called prior to any DeliverTxs.

#### EndBlock
  * __Arguments__:
    * `Height (uint64)`: The block height that ended
  * __Returns__:
    * `Validators ([]Validator)`: Changed validators with new voting powers (0 to remove)
  * __Usage__:<br/>
    Signals the end of a block.  Called prior to each Commit after all transactions

## Changelog

##### Mar 26h, 2016
* Introduce BeginBlock

##### Mar 6th, 2016

* Added InitChain, EndBlock

##### Feb 14th, 2016

* s/GetHash/Commit/g
* Document Protobuf request/response fields

##### Jan 23th, 2016

* Added CheckTx/Query ABCI message types
* Added Result/Log fields to DeliverTx/CheckTx/SetOption
* Removed Listener messages
* Removed Code from ResponseSetOption and ResponseGetHash
* Made examples BigEndian

##### Jan 12th, 2016

* Added "RetCodeBadNonce = 0x06" return code

##### Jan 8th, 2016

* Tendermint/ABCI now comes to consensus on the order first before DeliverTx.
