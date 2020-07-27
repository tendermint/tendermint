# Evidence Reactor

## Channels

[#1503](https://github.com/tendermint/tendermint/issues/1503)

Sending invalid evidence will result in stopping the peer.

Sending incorrectly encoded data or data exceeding `maxMsgSize` will result
in stopping the peer.
