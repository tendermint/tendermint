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

- [mempool] [\#4083](https://github.com/tendermint/tendermint/pull/4083) Added TxInfo parameter to CheckTx(), and removed CheckTxWithInfo() (@erikgrinaker)
- [mempool] [\#4057](https://github.com/tendermint/tendermint/issues/4057) Include peer ID when logging rejected txns (@erikgrinaker)
- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Improved `tm-monitor` formatting of start time and avg tx throughput (@erikgrinaker)
- [p2p] [\#3991](https://github.com/tendermint/tendermint/issues/3991) Log "has been established or dialed" as debug log instead of Error for connected peers (@whunmr)

### BUG FIXES:

- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Refresh `tm-monitor` health when validator count is updated (@erikgrinaker)
