## v0.32.7

\*\*

Special thanks to external contributors on this release:
@erikgrinaker

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

- [mempool] [\#4057](https://github.com/tendermint/tendermint/issues/4057) Include peer ID when logging rejected txns (@erikgrinaker)
- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Improved `tm-monitor` formatting of start time and avg tx throughput (@erikgrinaker)
- [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) No longer panic in `Query#Matches` preferring a log instead.
- [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) Strip out non-numeric characters when attempting to match numeric values.

### BUG FIXES:

- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Refresh `tm-monitor` health when validator count is updated (@erikgrinaker)
