# Tendermint Socket Protocol (TMSP)

Blockchains are a system for creating shared multi-master application state. 
**TMSP** is a socket protocol enabling a blockchain consensus engine, running in one process,
to manage a blockchain application state, running in another.

For more information on TMSP, motivations, and tutorials, please visit [our blog post](http://tendermint.com/posts/tendermint-socket-protocol/).

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
