# Applications

Please ensure you've first read the spec for [ABCI Methods and Types](abci.md)

Here we cover the following components of ABCI applications:

- [Connection State](#state) - the interplay between ABCI connections and application state
  and the differences between `CheckTx` and `DeliverTx`.
- [Transaction Results](#transaction-results) - rules around transaction
  results and validity
- [Validator Set Updates](#validator-updates) - how validator sets are
  changed during `InitChain` and `EndBlock`
- [Query](#query) - standards for using the `Query` method and proofs about the
  application state
- [Crash Recovery](#crash-recovery) - handshake protocol to synchronize
  Tendermint and the application on startup.

## State

Since Tendermint maintains three concurrent ABCI connections, it is typical
for an application to maintain a distinct state for each, and for the states to
be synchronized during `Commit`.

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
ie. the updates are linearizable.

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
latest committed block.

QueryState should be set to the latest `DeliverTxState` at the end of every `Commit`,
ie. after the full block has been processed and the state committed to disk.
Otherwise it should never be modified.

## Transaction Results

`ResponseCheckTx` and `ResponseDeliverTx` contain the same fields.

The `Info` and `Log` fields are non-deterministic values for debugging/convenience purposes
that are otherwise ignored.

The `Data` field must be strictly deterministic, but can be arbitrary data.

### Gas

Ethereum introduced the notion of `gas` as an abstract representation of the
cost of resources used by nodes when processing transactions. Every operation in the
Ethereum Virtual Machine uses some amount of gas, and gas can be accepted at a market-variable price.
Users propose a maximum amount of gas for their transaction; if the tx uses less, they get
the difference credited back. Tendermint adopts a similar abstraction,
though uses it only optionally and weakly, allowing applications to define
their own sense of the cost of execution.

In Tendermint, the `ConsensusParams.BlockSize.MaxGas` limits the amount of `gas` that can be used in a block.
The default value is `-1`, meaning no limit, or that the concept of gas is
meaningless.

Responses contain a `GasWanted` and `GasUsed` field. The former is the maximum
amount of gas the sender of a tx is willing to use, and the later is how much it actually
used. Applications should enforce that `GasUsed <= GasWanted` - ie. tx execution
should halt before it can use more resources than it requested.

When `MaxGas > -1`, Tendermint enforces the following rules:

- `GasWanted <= MaxGas` for all txs in the mempool
- `(sum of GasWanted in a block) <= MaxGas` when proposing a block

If `MaxGas == -1`, no rules about gas are enforced.

Note that Tendermint does not currently enforce anything about Gas in the consensus, only the mempool.
This means it does not guarantee that committed blocks satisfy these rules!
It is the application's responsibility to return non-zero response codes when gas limits are exceeded.

The `GasUsed` field is ignored completely by Tendermint. That said, applications should enforce:

- `GasUsed <= GasWanted` for any given transaction
- `(sum of GasUsed in a block) <= MaxGas` for every block

In the future, we intend to add a `Priority` field to the responses that can be
used to explicitly prioritize txs in the mempool for inclusion in a block
proposal. See [#1861](https://github.com/tendermint/tendermint/issues/1861).

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

Note that the maximum total power of the validator set is bounded by
`MaxTotalVotingPower = MaxInt64 / 8`. Applications are responsible for ensuring
they do not make changes to the validator set that cause it to exceed this
limit.

Additionally, applications must ensure that a single set of updates does not contain any duplicates -
a given public key can only appear in an update once. If an update includes
duplicates, the block execution will fail irrecoverably.

### InitChain

ResponseInitChain can return a list of validators.
If the list is empty, Tendermint will use the validators loaded in the genesis
file.
If the list is not empty, Tendermint will use it for the validator set.
This way the application can determine the initial validator set for the
blockchain.

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
- the total power of the new validator set must not exceed MaxTotalVotingPower

Note the updates returned in block `H` will only take effect at block `H+2`.

## Consensus Parameters

ConsensusParams enforce certain limits in the blockchain, like the maximum size
of blocks, amount of gas used in a block, and the maximum acceptable age of
evidence. They can be set in InitChain and updated in EndBlock.

### BlockSize.MaxBytes

The maximum size of a complete Amino encoded block.
This is enforced by Tendermint consensus.

This implies a maximum tx size that is this MaxBytes, less the expected size of
the header, the validator set, and any included evidence in the block.

Must have `0 < MaxBytes < 100 MB`.

### BlockSize.MaxGas

The maximum of the sum of `GasWanted` in a proposed block.
This is *not* enforced by Tendermint consensus.
It is left to the app to enforce (ie. if txs are included past the
limit, they should return non-zero codes). It is used by Tendermint to limit the
txs included in a proposed block.

Must have `MaxGas >= -1`.
If `MaxGas == -1`, no limit is enforced.

### EvidenceParams.MaxAge

This is the maximum age of evidence.
This is enforced by Tendermint consensus.
If a block includes evidence older than this, the block will be rejected
(validators won't vote for it).

Must have `0 < MaxAge`.

### Updates

The application may set the ConsensusParams during InitChain, and update them during
EndBlock. If the ConsensusParams is empty, it will be ignored. Each field
that is not empty will be applied in full. For instance, if updating the
BlockSize.MaxBytes, applications must also set the other BlockSize fields (like
BlockSize.MaxGas), even if they are unchanged, as they will otherwise cause the
value to be updated to 0.

#### InitChain

ResponseInitChain includes a ConsensusParams.
If its nil, Tendermint will use the params loaded in the genesis
file. If it's not nil, Tendermint will use it.
This way the application can determine the initial consensus params for the
blockchain.

#### EndBlock

ResponseEndBlock includes a ConsensusParams.
If its nil, Tendermint will do nothing.
If it's not nil, Tendermint will use it.
This way the application can update the consensus params over time.

Note the updates returned in block `H` will take effect right away for block
`H+1`.

## Query

Query is a generic method with lots of flexibility to enable diverse sets
of queries on application state. Tendermint makes use of Query to filter new peers
based on ID and IP, and exposes Query to the user over RPC.

Note that calls to Query are not replicated across nodes, but rather query the
local node's state - hence they may return stale reads. For reads that require
consensus, use a transaction.

The most important use of Query is to return Merkle proofs of the application state at some height
that can be used for efficient application-specific lite-clients.

Note Tendermint has technically no requirements from the Query
message for normal operation - that is, the ABCI app developer need not implement
Query functionality if they do not wish too.

### Query Proofs

The Tendermint block header includes a number of hashes, each providing an
anchor for some type of proof about the blockchain. The `ValidatorsHash` enables
quick verification of the validator set, the `DataHash` gives quick
verification of the transactions included in the block, etc.

The `AppHash` is unique in that it is application specific, and allows for
application-specific Merkle proofs about the state of the application.
While some applications keep all relevant state in the transactions themselves
(like Bitcoin and its UTXOs), others maintain a separated state that is
computed deterministically *from* transactions, but is not contained directly in
the transactions themselves (like Ethereum contracts and accounts).
For such applications, the `AppHash` provides a much more efficient way to verify lite-client proofs.

ABCI applications can take advantage of more efficient lite-client proofs for
their state as follows:

- return the Merkle root of the deterministic application state in
`ResponseCommit.Data`.
- it will be included as the `AppHash` in the next block.
- return efficient Merkle proofs about that application state in `ResponseQuery.Proof`
  that can be verified using the `AppHash` of the corresponding block.

For instance, this allows an application's lite-client to verify proofs of
absence in the application state, something which is much less efficient to do using the block hash.

Some applications (eg. Ethereum, Cosmos-SDK) have multiple "levels" of Merkle trees,
where the leaves of one tree are the root hashes of others. To support this, and
the general variability in Merkle proofs, the `ResponseQuery.Proof` has some minimal structure:

```
message Proof {
  repeated ProofOp ops
}

message ProofOp {
  string type = 1;
  bytes key = 2;
  bytes data = 3;
}
```

Each `ProofOp` contains a proof for a single key in a single Merkle tree, of the specified `type`.
This allows ABCI to support many different kinds of Merkle trees, encoding
formats, and proofs (eg. of presence and absence) just by varying the `type`.
The `data` contains the actual encoded proof, encoded according to the `type`.
When verifying the full proof, the root hash for one ProofOp is the value being
verified for the next ProofOp in the list. The root hash of the final ProofOp in
the list should match the `AppHash` being verified against.

### Peer Filtering

When Tendermint connects to a peer, it sends two queries to the ABCI application
using the following paths, with no additional data:

- `/p2p/filter/addr/<IP:PORT>`, where `<IP:PORT>` denote the IP address and
  the port of the connection
- `p2p/filter/id/<ID>`, where `<ID>` is the peer node ID (ie. the
  pubkey.Address() for the peer's PubKey)

If either of these queries return a non-zero ABCI code, Tendermint will refuse
to connect to the peer.

### Paths

Queries are directed at paths, and may optionally include additional data.

The expectation is for there to be some number of high level paths
differentiating concerns, like `/p2p`, `/store`, and `/app`. Currently,
Tendermint only uses `/p2p`, for filtering peers. For more advanced use, see the
implementation of
[Query in the Cosmos-SDK](https://github.com/cosmos/cosmos-sdk/blob/v0.23.1/baseapp/baseapp.go#L333).

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
    completed Commit
```

Note we always have `storeBlockHeight >= stateBlockHeight` and `storeBlockHeight >= appBlockHeight`
Note also we never call Commit on an ABCI app twice for the same height.

The procedure is as follows.

First, some simple start conditions:

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

If `storeBlockHeight == stateBlockHeight && appBlockHeight == storeBlockHeight`, we're done.
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

If `appBlockHeight == storeBlockHeight`
    update the state using the saved ABCI responses but dont run the block against the real app.
This happens if we crashed after the app finished Commit but before Tendermint saved the state.

