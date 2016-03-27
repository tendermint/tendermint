# Tendermint Socket Protocol (TMSP)

Blockchains are a system for creating shared multi-master application state. 
**TMSP** is a socket protocol enabling a blockchain consensus engine, running in one process,
to manage a blockchain application state, running in another.

For more information on TMSP, motivations, and tutorials, please visit [our blog post](http://tendermint.com/posts/tendermint-socket-protocol/).

## Message types

TMSP requests/responses are simple Protobuf messages.  Check out the [schema file](https://github.com/tendermint/tmsp/blob/master/types/types.proto).

#### AppendTx
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
    Validate a transaction.  This message should not mutate the state.
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
    Signals the beginning of a new block. Called prior to any AppendTxs.

#### EndBlock
  * __Arguments__:
    * `Height (uint64)`: The block height that ended
  * __Returns__:
    * `Validators ([]Validator)`: Changed validators with new voting powers (0 to remove)
  * __Usage__:<br/>
    Signals the end of a block.  Called prior to each Commit after all transactions

### Changelog

#### Mar 26h, 2016
* Introduce BeginBlock

#### Mar 6th, 2016

* Added InitChain, EndBlock

#### Feb 14th, 2016

* s/GetHash/Commit/g
* Document Protobuf request/response fields

#### Jan 23th, 2016

* Added CheckTx/Query TMSP message types
* Added Result/Log fields to AppendTx/CheckTx/SetOption
* Removed Listener messages
* Removed Code from ResponseSetOption and ResponseGetHash
* Made examples BigEndian

#### Jan 12th, 2016

* Added "RetCodeBadNonce = 0x06" return code

#### Jan 8th, 2016

* Tendermint/TMSP now comes to consensus on the order first before AppendTx.
