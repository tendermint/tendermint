# Unreleased Changes

## v0.34.12

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- [rpc] [\#6717](https://github.com/tendermint/tendermint/pull/6717) introduce
  `/genesis_chunked` rpc endpoint for handling large genesis files by chunking them

### IMPROVEMENTS

### BUG FIXES

- [light] [\#6685](https://github.com/tendermint/tendermint/pull/6685) fix bug
  with incorrectly handling contexts that would occasionally freeze state sync. (@cmwaters)

