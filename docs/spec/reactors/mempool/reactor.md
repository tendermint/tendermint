# Mempool Reactor

## Channels

[#1503](https://github.com/tendermint/tendermint/issues/1503)

Mempool maintains a cache of the last 10000 transactions to prevent
replaying old transactions (plus transactions coming from other
validators, who are continually exchanging transactions). Read [Replay
Protection](https://tendermint.readthedocs.io/projects/tools/en/master/app-development.html?#replay-protection)
for details.

Sending incorrectly encoded data or data exceeding `maxMsgSize` will result
in stopping the peer.
