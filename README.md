# Tendermint Streaming Protocol (TMSP)

**TMSP** is a socket protocol, which means applications can be written in any programming language.
TMSP is an asynchronous streaming protocol: message responses are written back asynchronously to the platform.

*Applications must be deterministic.*

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

#### SetEventsMode
  * __Arguments__:
    * `EventsMode (int8)`:
      * `EventsModeOff (0)`: Events are not reported. Used for mempool.
      * `EventsModeCached (1)`: Events are cached.
      * `EventsModeOn (2)`: Flush cache and report events.
  * __Returns__:
    * `RetCode (int8)`
  * __Usage__:<br/>
    Set event reporting mode for future transactions

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

