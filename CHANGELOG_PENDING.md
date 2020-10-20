# Unreleased Changes

## v0.34.0-rc6

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [statesync] \#5516 Check that all heights necessary to rebuild state for a snapshot exist before adding the snapshot to the pool. (@erikgrinaker)

### BUG FIXES

- [blockchain/v2] \#5499 Fix "duplicate block enqueued by processor" panic (@melekes)
- [abci/grpc] \#5520 Return async responses in order, to avoid mempool panics. (@erikgrinaker)
