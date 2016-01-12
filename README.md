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

#### CheckTx
  * __Arguments__:
    * `TxBytes ([]byte)`
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Validate a transaction.  This message should not mutate the state.

#### GetHash
  * __Returns__:
    * `RetCode (int8)`
    * `Hash ([]byte)`
  * __Usage__:<br/>
    Return a Merkle root hash of the application state

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

## Changelog

### Jan 12th, 2016

* Added "RetCodeBadNonce = 0x06" return code

### Jan 8th, 2016

Tendermint/TMSP now comes to consensus on the order first before AppendTx.
This means that we no longer need the Commit/Rollback TMSP messages.
Instead, there’s a “CheckTx” message for mempool to check the validity of a message.
One consequence is that txs in blocks now may include invalid txs that are ignored.
In the future, we can include a bitarray or merkle structure in the block so anyone can see which txs were valid.
To prevent spam, applications can implement their “CheckTx” messages to deduct some balance, so at least spam txs will cost something.  This isn’t any more work that what we already needed to do, so it’s not any worse.
You can see the new changes in the tendermint/tendermint “order_first” branch, and tendermint/tmsp “order_first” branch.  If you your TMSP apps to me I can help with the transition.
Please take a look at how the examples in TMSP changed, e.g. how AppContext was removed, CheckTx was added, how the TMSP msg bytes changed, and how commit/rollback messages were removed.
