## v0.33.2

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

  - [p2p] [\#4449](https://github.com/tendermint/tendermint/pull/4449) Removed `conn.ErrSharedSecretIsZero`. A generic error is returned for this condition instead. (@erikgrinaker)

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:

- [rpc] [\#4437](https://github.com/tendermint/tendermint/pull/4437) Fix tx_search pagination with ordered results (@erikgrinaker)

- [rpc] [\#4406](https://github.com/tendermint/tendermint/pull/4406) Fix issue with multiple subscriptions on the websocket (@antho1404)
