---
order: 1
parent:
  title: Mempool
  order: 2
---

The mempool is a in memory pool of potentially valid transactions,
both to broadcast to other nodes, as well as to provide to the
consensus reactor when it is selected as the block proposer.

There are two sides to the mempool state:

- External: get, check, and broadcast new transactions
- Internal: return valid transaction, update list after block commit

## External functionality

External functionality is exposed via network interfaces
to potentially untrusted actors.

- CheckTx - triggered via RPC or P2P
- Broadcast - gossip messages after a successful check

## Internal functionality

Internal functionality is exposed via method calls to other
code compiled into the tendermint binary.

- ReapMaxBytesMaxGas - get txs to propose in the next block. Guarantees that the
    size of the txs is less than MaxBytes, and gas is less than MaxGas
- Update - remove tx that were included in last block
- ABCI.CheckTx - call ABCI app to validate the tx

What does it provide the consensus reactor?
What guarantees does it need from the ABCI app?
(talk about interleaving processes in concurrency)

## Optimizations

The implementation within this library also implements a tx cache.
This is so that signatures don't have to be reverified if the tx has
already been seen before.
However, we only store valid txs in the cache, not invalid ones.
This is because invalid txs could become good later.
Txs that are included in a block aren't removed from the cache,
as they still may be getting received over the p2p network.
These txs are stored in the cache by their hash, to mitigate memory concerns.

Applications should implement replay protection, read [Replay
Protection](https://github.com/tendermint/tendermint/blob/8cdaa7f515a9d366bbc9f0aff2a263a1a6392ead/docs/app-dev/app-development.md#replay-protection) for more information.

## Configuration

The mempool has various configurable paramet

Sending incorrectly encoded data or data exceeding `maxMsgSize` will result
in stopping the peer.

`maxMsgSize` equals `MaxBatchBytes` (10MB) + 4 (proto overhead).
`MaxBatchBytes` is a mempool config parameter -> defined locally. The reactor
sends transactions to the connected peers in batches. The maximum size of one
batch is `MaxBatchBytes`.

The mempool will not send a tx back to any peer which it received it from.

The reactor assigns an `uint16` number for each peer and maintains a map from
p2p.ID to `uint16`. Each mempool transaction carries a list of all the senders
(`[]uint16`). The list is updated every time mempool receives a transaction it
is already seen. `uint16` assumes that a node will never have over 65535 active
peers (0 is reserved for unknown source - e.g. RPC).
