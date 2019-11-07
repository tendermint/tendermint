## v0.32.8

\*\*

Special thanks to external contributors on this release:
@erikgrinaker

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API
  - [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) `Query#(Matches|Conditions)` returns an error.

### FEATURES:

### IMPROVEMENTS:

- [mempool] [\#4083](https://github.com/tendermint/tendermint/pull/4083) Added TxInfo parameter to CheckTx(), and removed CheckTxWithInfo() (@erikgrinaker)
- [mempool] [\#4057](https://github.com/tendermint/tendermint/issues/4057) Include peer ID when logging rejected txns (@erikgrinaker)
- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Improved `tm-monitor` formatting of start time and avg tx throughput (@erikgrinaker)
- [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) No longer panic in `Query#(Matches|Conditions)` preferring to return an error instead.
- [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) Strip out non-numeric characters when attempting to match numeric values.
- [p2p] [\#3991](https://github.com/tendermint/tendermint/issues/3991) Log "has been established or dialed" as debug log instead of Error for connected peers (@whunmr)
- [privval] Add `SignerDialerEndpointRetryWaitInterval` option (@cosmostuba)

### BUG FIXES:

- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Refresh `tm-monitor` health when validator count is updated (@erikgrinaker)
- [state] [\#4104](https://github.com/tendermint/tendermint/pull/4104) txindex/kv: Fsync data to disk immediately after receiving it (@guagualvcha)
- [state] [\#4095](https://github.com/tendermint/tendermint/pull/4095) txindex/kv: Return an error if there's one when the user searches for a tx (hash=X) (@hsyis)
