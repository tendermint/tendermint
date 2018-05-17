# Mempool Functionality

The mempool maintains a list of potentially valid transactions,
both to broadcast to other nodes, as well as to provide to the
consensus reactor when it is selected as the block proposer.

There are two sides to the mempool state:

* External: get, check, and broadcast new transactions
* Internal: return valid transaction, update list after block commit


## External functionality

External functionality is exposed via network interfaces
to potentially untrusted actors.

* CheckTx - triggered via RPC or P2P
* Broadcast - gossip messages after a successful check

## Internal functionality

Internal functionality is exposed via method calls to other
code compiled into the tendermint binary.

* Reap - get tx to propose in next block
* Update - remove tx that were included in last block
* ABCI.CheckTx - call ABCI app to validate the tx

What does it provide the consensus reactor?
What guarantees does it need from the ABCI app?
(talk about interleaving processes in concurrency)

## Optimizations

Talk about the LRU cache to make sure we don't process any
tx that we have seen before
