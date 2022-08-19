# Mempool Reactor

## Channels

See [this issue](https://github.com/tendermint/tendermint/issues/1503)

Mempool maintains a cache of the last 10000 transactions to prevent
replaying old transactions (plus transactions coming from other
validators, who are continually exchanging transactions). Read [Replay
Protection](https://github.com/tendermint/tendermint/blob/8cdaa7f515a9d366bbc9f0aff2a263a1a6392ead/docs/app-dev/app-development.md#replay-protection)
for details.

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
