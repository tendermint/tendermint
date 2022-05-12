# Unreleased Changes

## v0.34.20

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- [cli] [\#8674] Add command to force compact goleveldb databases (@cmwaters)

### IMPROVEMENTS

### BUG FIXES

- [blocksync] [\#8496](https://github.com/tendermint/tendermint/pull/8496) validate block against state before persisting it to disk (@cmwaters)
- (indexer) [#8625](https://github.com/tendermint/tendermint/pull/8625) Fix overriding tx index of duplicated txs.
