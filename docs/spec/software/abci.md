# Application Blockchain Interface (ABCI)

ABCI is the interface between Tendermint (a state-machine replication engine)
and an application (the actual state machine).

The ABCI message types are defined in a [protobuf
file](https://github.com/tendermint/tendermint/blob/develop/abci/types/types.proto).

For full details on the ABCI message types and protocol, see the [ABCI
specification](https://github.com/tendermint/tendermint/blob/develop/docs/abci-spec.md).
Be sure to read the specification if you're trying to build an ABCI app!

For additional details on server implementation, see the [ABCI
readme](https://github.com/tendermint/tendermint/blob/develop/abci/README.md).

Here we provide some more details around the use of ABCI by Tendermint and
clarify common "gotchas".

## ABCI connections

Tendermint opens 3 ABCI connections to the app: one for Consensus, one for
Mempool, one for Queries.

## Async vs Sync

The main ABCI server (ie. non-GRPC) provides ordered asynchronous messages.
This is useful for DeliverTx and CheckTx, since it allows Tendermint to forward
transactions to the app before it's finished processing previous ones.

Thus, DeliverTx and CheckTx messages are sent asycnhronously, while all other
messages are sent synchronously.

## CheckTx and Commit

It is typical to hold three distinct states in an ABCI app: CheckTxState, DeliverTxState,
QueryState. The QueryState contains the latest committed state for a block.
The CheckTxState and DeliverTxState may be updated concurrently with one another.
Before Commit is called, Tendermint locks and flushes the mempool so that no new changes will happen
to CheckTxState. When Commit completes, it unlocks the mempool.

Thus, during Commit, it is safe to reset the QueryState and the CheckTxState to the latest DeliverTxState
(ie. the new state from executing all the txs in the block).

Note, however, that it is not possible to send transactions to Tendermint during Commit - if your app
tries to send a `/broadcast_tx` to Tendermint during Commit, it will deadlock.


## EndBlock Validator Updates

Updates to the Tendermint validator set can be made by returning `Validator`
objects in the `ResponseBeginBlock`:

```
message Validator {
  bytes address = 1;
  PubKey pub_key = 2;
  int64 power = 3;
}

message PubKey {
  string type = 1;
  bytes  data = 2;
}

```

The `pub_key` currently supports two types:
    - `type = "ed25519" and `data = <raw 32-byte public key>`
    - `type = "secp256k1" and `data = <33-byte OpenSSL compressed public key>`

If the address is provided, it must match the address of the pubkey, as
specified [here](/docs/spec/blockchain/encoding.md#Addresses)

(Note: In the v0.19 series, the `pub_key` is the [Amino encoded public
key](/docs/spec/blockchain/encoding.md#public-key-cryptography).
For Ed25519 pubkeys, the Amino prefix is always "1624DE6220". For example, the 32-byte Ed25519 pubkey
`76852933A4686A721442E931A8415F62F5F1AEDF4910F1F252FB393F74C40C85` would be
Amino encoded as
`1624DE622076852933A4686A721442E931A8415F62F5F1AEDF4910F1F252FB393F74C40C85`)

(Note: In old versions of Tendermint (pre-v0.19.0), the pubkey is just prefixed with a
single type byte, so for ED25519 we'd have `pub_key = 0x1 | pub`)

The `power` is the new voting power for the validator, with the
following rules:

- power must be non-negative
- if power is 0, the validator must already exist, and will be removed from the
  validator set
- if power is non-0:
    - if the validator does not already exist, it will be added to the validator
      set with the given power
    - if the validator does already exist, its power will be adjusted to the given power

## InitChain Validator Updates

ResponseInitChain has the option to return a list of validators.
If the list is not empty, Tendermint will adopt it for the validator set.
This way the application can determine the initial validator set for the
blockchain.

Note that if addressses are included in the returned validators, they must match
the address of the public key.

ResponseInitChain also includes ConsensusParams, but these are presently
ignored.

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

## Info and the Handshake/Replay

On startup, Tendermint calls Info on the Query connection to get the latest
committed state of the app. The app MUST return information consistent with the
last block it succesfully completed Commit for.

If the app succesfully committed block H but not H+1, then `last_block_height =
H` and `last_block_app_hash = <hash returned by Commit for block H>`. If the app
failed during the Commit of block H, then `last_block_height = H-1` and
`last_block_app_hash = <hash returned by Commit for block H-1, which is the hash
in the header of block H>`.

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
