# Unreleased Changes

## v0.34.8

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

- [libs/log] \#6174 Include timestamp (`ts` field; `time.RFC3339Nano` format) in JSON logger output (@melekes)

### BUG FIXES

- [ABCI] \#6124 Fixes a panic condition during callback execution in `ReCheckTx` during high tx load. (@alexanderbez)
