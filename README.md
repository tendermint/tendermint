# Tendermint Socket Protocol (TMSP)

**TMSP** is a socket protocol enabling a consensus engine, running in one process,
to manage an application state, running in another.
Thus the applications can be written in any programming language.

TMSP is an asynchronous protocol: message responses are written back asynchronously to the platform.

*Applications must be deterministic.*

For more information on TMSP, motivations, and tutorials, please visit [our blog post](http://tendermint.com/posts/tendermint-socket-protocol/).

## Intro to TMSP

The Tendermint Socket Protocol (TMSP) and Tendermint Core are a set of tools for building blockchain based databases. Blockchains are a system for creating shared multi-master application state. The Tendermint Core approach is to decouple the consensus engine and P2P layer from the details of the application state of the particular blockchain.

To draw an analogy, lets talk about a well-known cryptocurrency, Bitcoin.  Bitcoin creates a cryptocurrency from a blockchain by having each full node maintain a fully audited Unspent Transaction Output (UTXO) database. If one wanted to create a Bitcoin like system on top of TMSP, Tendermint Core would be responsible for 

- Sharing blocks and transactions between nodes
- Establishing a canonical/immutable order of transactions (the blockchain)

The Applications developer will be responsible for

- Maintaining the UTXO database
- Validating cryptographic signatures of transactions
- Preventing transactions from spending non-existent transactions
Allowing clients to query the UTXO database.

Tendermint is able to decompose the blockchain design by offering a very simple API between the application layer and consensus layer.

The API consists of 4 primary message types from the consensus engine that the application layer needs to respond to.

The messages are specified here:
https://github.com/tendermint/tmsp#message-types

The AppendTx message type is the work horse of the Application. Each transaction from the blockchain is delivered with this message. The developer needs to validate each transactions received with the AppendTx message against the current state, application protocol, and the cryptographic credentials of the transaction. A validated transaction then needs to update the application state — by binding a value into a key values store, or by updating the UTXO database.

The purpose of the GetHash message is to allow a cryptographic commitment to the current application state to be placed into the next block header. This has some handy properties. Inconsistencies in updating that state will now appear as blockchain forks which catches a whole class of programming errors. This also simplifies the development of secure lightweight clients.

Finally, in order to recover from faulty proposers (e.g. a malicious validator that proposes two conflicting blocks), the application must be able to roll back transactions.  The Commit message tells the application to create a checkpoint, and the Rollback message to unwind the state back to that checkpoint.

There can be multiple TMSP socket connections to an application.  Each connection has an isolated “view” of the application state, but Commit/Rollback work across connections. Application developers can store the application state in an immutable data structure (such as Tendermint’s Go-Merkle library) to make this easy to develop.

NOTE: Tendermint Core creates two TMSP connections to the application; one for the validation of transactions when broadcasting in the mempool layer, and another for the consensus engine to run block proposals.  A commit on one connection followed by a rollback on the other syncs both views.

It's probably evident that applications designers need to very carefully design their message handlers to create a blockchain that does anything useful but this architecture provides a place to start.

## Message types

#### AppendTx
  * __Arguments__:
    * `TxBytes ([]byte)`
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Append and run a transaction.  The transaction may or may not be final.

#### GetHash
  * __Returns__:
    * `RetCode (int8)`
    * `Hash ([]byte)`
  * __Usage__:<br/>
    Return a Merkle root hash of the application state

#### Commit
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Finalize all appended transactions

#### Rollback
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Roll back to the last commit

#### AddListener
  * __Arguments__:
    * `EventKey (string)`
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Add event listener callback for events with given key.

#### RemoveListener
  * __Arguments__:
    * `EventKey (string)`
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Remove event listener callback for events with given key.

#### Flush
  * __Usage__:<br/>
    Flush the response queue.  Applications that implement `types.Application` need not implement this message -- it's handled by the project.

#### Info
  * __Returns__:
    * `Data ([]string)`
  * __Usage__:<br/>
    Return an array of strings about the application state.  Application specific.

#### SetOption
  * __Arguments__:
    * `Key (string)`
    * `Value (string)`
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Set application options.  E.g. Key="mode", Value="mempool" for a mempool connection, or Key="mode", Value="consensus" for a consensus connection.
    Other options are application specific.

## Contributions

* Thanks to Zaki Manian for the introductory text.
