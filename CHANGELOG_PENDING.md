## v0.33.2

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

- [types] [\#4417](https://github.com/tendermint/tendermint/issues/4417) VerifyCommitX() functions should return as soon as +2/3 threashold is reached.
- [examples/kvstore] [\#4509](https://github.com/tendermint/tendermint/pull/4509) ABCI query now returns the proper height (@erikgrinaker)
- [cmd] \#4515 Change `tendermint debug dump` sub-command archives filename's format (@melekes)

### BUG FIXES:

- [rpc] \#3935 Create buffered subscriptions on `/subscribe` (@melekes)
- [rpc] [\#4493](https://github.com/tendermint/tendermint/pull/4493) Keep the original subscription "id" field when new RPCs come in (@michaelfig)
- [rpc] [\#4437](https://github.com/tendermint/tendermint/pull/4437) Fix tx_search pagination with ordered results (@erikgrinaker)
- [rpc] [\#4406](https://github.com/tendermint/tendermint/pull/4406) Fix issue with multiple subscriptions on the websocket (@antho1404)
- [cmd] \#4515 Fix `tendermint debug kill` sub-command (@melekes)
