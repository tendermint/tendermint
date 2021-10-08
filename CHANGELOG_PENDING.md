# Unreleased Changes

## v0.34.14

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

<<<<<<< HEAD
- [\#6982](https://github.com/tendermint/tendermint/pull/6982) tendermint binary has built-in suppport for running the end to end application (with state sync support) (@cmwaters).
=======
- [cli] [#7033](https://github.com/tendermint/tendermint/pull/7033) Add a `rollback` command to rollback to the previous tendermint state in the event of non-determinstic app hash or reverting an upgrade.
- [mempool, rpc] \#7041  Add removeTx operation to the RPC layer. (@tychoish)
>>>>>>> 4ca130d22 (cli: allow node operator to rollback last state (#7033))

### IMPROVEMENTS

### BUG FIXES

- [\#7057](https://github.com/tendermint/tendermint/pull/7057) Import Postgres driver support for the psql indexer (@creachadair).
