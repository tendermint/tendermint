# Mempool Reactor

## Channels

See [this issue](https://github.com/tendermint/tendermint/issues/1503)

Mempool maintains a cache of the last 10000 transactions to prevent
replaying old transactions (plus transactions coming from other
validators, who are continually exchanging transactions). Read [Replay
Protection](../../../app-dev/app-development.md#replay-protection)
for details.

Sending incorrectly encoded data or data exceeding `maxMsgSize` will result
in stopping the peer.

The mempool will not send a tx back to any peer which it received it from.

The reactor assigns an `uint16` number for each peer and maintains a map from
p2p.ID to `uint16`. Each mempool transaction carries a list of all the senders
(`[]uint16`). The list is updated every time mempool receives a transaction it
is already seen. `uint16` assumes that a node will never have over 65535 active
peers (0 is reserved for unknown source - e.g. RPC).
