# Applications

Please ensure you've first read the spec for [ABCI Methods and Types](abci.md)

Here we cover the following components of ABCI applications:

- [State](#state) - the interplay between ABCI connections and application state
  and the differences between `CheckTx` and `DeliverTx`.
- [Transaction Results](#transaction-results) - rules around transaction
  results and validity
- [Validator Set Updates](#validator-updates) - how validator sets are
  changed during `InitChain` and `EndBlock`
- [Query](#query) - standards for using the `Query` method
- [Crash Recovery](#crash-recovery) - handshake protocol to synchronize
  Tendermint and the application on startup.

## State

Since Tendermint maintains multiple concurrent ABCI connections, it is typical
for an application to maintain a distinct state for each, and for the states to
be sycnronized during `Commit`.

### Commit

Application state should only be persisted to disk during `Commit`.

Before `Commit` is called, Tendermint locks and flushes the mempool so that no new messages will
be received on the mempool connection. This provides an opportunity to safely update all three
states to the latest committed state at once.

When `Commit` completes, it unlocks the mempool.

Note that it is not possible to send transactions to Tendermint during `Commit` - if your app
tries to send a `/broadcast_tx` to Tendermint during Commit, it will deadlock.

### Consensus Connection

The Consensus Connection should maintain a `DeliverTxState` -
the working state for block execution. It should be updated by the calls to
`BeginBlock`, `DeliverTx`, and `EndBlock` during block execution and committed to
disk as the "latest committed state" during `Commit`.

Updates made to the DeliverTxState by each method call must be readable by each subsequent method -
ie. the updates are linearizeable.

### Mempool Connection

The Mempool Connection should maintain a `CheckTxState`
to sequentially process pending transactions in the mempool that have
not yet been committed. It should be initialized to the latest committed state
at the end of every `Commit`.

The CheckTxState may be updated concurrently with the DeliverTxState, as
messages may be sent concurrently on the Consensus and Mempool connections. However,
before calling `Commit`, Tendermint will lock and flush the mempool connection,
ensuring that all existing CheckTx are responded to and no new ones can
begin.

After `Commit`, CheckTx is run again on all transactions that remain in the
node's local mempool after filtering those included in the block. To prevent the
mempool from rechecking all transactions every time a block is committed, set
the configuration option `mempool.recheck=false`.

Finally, the mempool will unlock and new transactions can be processed through CheckTx again.

Note that CheckTx doesn't have to check everything that affects transaction validity; the
expensive things can be skipped. In fact, CheckTx doesn't have to check
anything; it might say that any transaction is a valid transaction.
Unlike DeliverTx, CheckTx is just there as
a sort of weak filter to keep invalid transactions out of the blockchain. It's
weak, because a Byzantine node doesn't care about CheckTx; it can propose a
block full of invalid transactions if it wants.

### Info Connection

The Info Connection should maintain a `QueryState` for answering queries from the user,
and for initialization when Tendermint first starts up (both described further
below).
It should always contain the latest committed state associated with the
latest commited block.

QueryState should be set to the latest `DeliverTxState` at the end of every `Commit`,
ie. after the full block has been processed and the state committed to disk.
Otherwise it should never be modified.

## Transaction Results

`ResponseCheckTx` and `ResponseDeliverTx` contain the same fields, though they
have slightly different effects.

In both cases, `Info` and `Log` are non-deterministic values for debugging/convenience purposes
that are otherwise ignored.

In both cases, `GasWanted` and `GasUsed` parameters are currently ignored,
though see issues
[#1861](https://github.com/tendermint/tendermint/issues/1861),
[#2299](https://github.com/tendermint/tendermint/issues/2299) and
[#2310](https://github.com/tendermint/tendermint/issues/2310) for how this may
change.

### CheckTx

If `Code != 0`, it will be rejected from the mempool and hence
not broadcasted to other peers and not included in a proposal block.

`Data` contains the result of the CheckTx transaction execution, if any. It is
semantically meaningless to Tendermint.

`Tags` include any tags for the execution, though since the transaction has not
been committed yet, they are effectively ignored by Tendermint.

### DeliverTx

If DeliverTx returns `Code != 0`, the transaction will be considered invalid,
though it is still included in the block.

`Data` contains the result of the CheckTx transaction execution, if any. It is
semantically meaningless to Tendermint.

Both the `Code` and `Data` are included in a structure that is hashed into the
`LastResultsHash` of the next block header.

`Tags` include any tags for the execution, which Tendermint will use to index
the transaction by. This allows transactions to be queried according to what
events took place during their execution.

See issue [#1007](https://github.com/tendermint/tendermint/issues/1007) for how
the tags will be hashed into the next block header.

## Validator Updates

The application may set the validator set during InitChain, and update it during
EndBlock.

### InitChain

ResponseInitChain can return a list of validators.
If the list is empty, Tendermint will use the validators loaded in the genesis
file.
If the list is not empty, Tendermint will use it for the validator set.
This way the application can determine the initial validator set for the
blockchain.

ResponseInitChain also includes ConsensusParams, but these are presently
ignored.

### EndBlock

Updates to the Tendermint validator set can be made by returning
`ValidatorUpdate` objects in the `ResponseEndBlock`:

```
message ValidatorUpdate {
  PubKey pub_key
  int64 power
}

message PubKey {
  string type
  bytes  data
}
```

The `pub_key` currently supports only one type:

- `type = "ed25519" and`data = <raw 32-byte public key>`

The `power` is the new voting power for the validator, with the
following rules:

- power must be non-negative
- if power is 0, the validator must already exist, and will be removed from the
  validator set
- if power is non-0:
  - if the validator does not already exist, it will be added to the validator
    set with the given power
  - if the validator does already exist, its power will be adjusted to the given power

Note the updates returned in block `H` will only take effect at block `H+2`.

## Query

Query is a generic message type with lots of flexibility to enable diverse sets
of queries from applications. Tendermint has no requirements from the Query
message for normal operation - that is, the ABCI app developer need not implement Query functionality if they do not wish too.
That said, Tendermint makes a number of queries to support some optional
features. These are:

### Peer Filtering

When Tendermint connects to a peer, it sends two queries to the ABCI application
using the following paths, with no additional data:

- `/p2p/filter/addr/<IP:PORT>`, where `<IP:PORT>` denote the IP address and
  the port of the connection
- `p2p/filter/id/<ID>`, where `<ID>` is the peer node ID (ie. the
  pubkey.Address() for the peer's PubKey)

If either of these queries return a non-zero ABCI code, Tendermint will refuse
to connect to the peer.


## Crash Recovery

On startup, Tendermint calls the `Info` method on the Info Connection to get the latest
committed state of the app. The app MUST return information consistent with the
last block it succesfully completed Commit for.

If the app succesfully committed block H but not H+1, then `last_block_height = H` and `last_block_app_hash = <hash returned by Commit for block H>`. If the app
failed during the Commit of block H, then `last_block_height = H-1` and
`last_block_app_hash = <hash returned by Commit for block H-1, which is the hash in the header of block H>`.

We now distinguish three heights, and describe how Tendermint syncs itself with
the app.

```
storeBlockHeight = height of the last block Tendermint saw a commit for
stateBlockHeight = height of the last block for which Tendermint completed all
    block processing and saved all ABCI results to disk
appBlockHeight = height of the last block for which ABCI app succesfully
    completely Commit
```

Note we always have `storeBlockHeight >= stateBlockHeight` and `storeBlockHeight >= appBlockHeight`
Note also we never call Commit on an ABCI app twice for the same height.

The procedure is as follows.

First, some simeple start conditions:

If `appBlockHeight == 0`, then call InitChain.

If `storeBlockHeight == 0`, we're done.

Now, some sanity checks:

If `storeBlockHeight < appBlockHeight`, error
If `storeBlockHeight < stateBlockHeight`, panic
If `storeBlockHeight > stateBlockHeight+1`, panic

Now, the meat:

If `storeBlockHeight == stateBlockHeight && appBlockHeight < storeBlockHeight`,
replay all blocks in full from `appBlockHeight` to `storeBlockHeight`.
This happens if we completed processing the block, but the app forgot its height.

If `storeBlockHeight == stateBlockHeight && appBlockHeight == storeBlockHeight`, we're done
This happens if we crashed at an opportune spot.

If `storeBlockHeight == stateBlockHeight+1`
This happens if we started processing the block but didn't finish.

    If `appBlockHeight < stateBlockHeight`
    	replay all blocks in full from `appBlockHeight` to `storeBlockHeight-1`,
    	and replay the block at `storeBlockHeight` using the WAL.
    This happens if the app forgot the last block it committed.

    If `appBlockHeight == stateBlockHeight`,
    	replay the last block (storeBlockHeight) in full.
    This happens if we crashed before the app finished Commit

    If appBlockHeight == storeBlockHeight {
    	update the state using the saved ABCI responses but dont run the block against the real app.
    This happens if we crashed after the app finished Commit but before Tendermint saved the state.
