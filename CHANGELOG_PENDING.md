# Unreleased Changes

## v0.34.15

Special thanks to external contributors on this release: @thanethomson

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

  - [config] [\#7230](https://github.com/tendermint/tendermint/issues/7230) rpc: Add experimental config params to allow for subscription buffer size control (@thanethomson).

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

### BUG FIXES

- [\#7309](https://github.com/tendermint/tendermint/issues/7309) pubsub: Report a non-nil error when shutting down (fixes #7306).
- [\#7057](https://github.com/tendermint/tendermint/pull/7057) Import Postgres driver support for the psql indexer (@creachadair).
- [\#7106](https://github.com/tendermint/tendermint/pull/7106) Revert mutex change to ABCI Clients (@tychoish).
