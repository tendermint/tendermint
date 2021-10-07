# Unreleased Changes

Friendly reminder: We have a [bug bounty program](https://hackerone.com/cosmos).

## vX.X

Month, DD, YYYY

Special thanks to external contributors on this release:

### BREAKING CHANGES

- CLI/RPC/Config

- Apps
  - [proto/tendermint] \#6976 Remove core protobuf files in favor of only housing them in the [tendermint/spec](https://github.com/tendermint/spec) repository.

- P2P Protocol

  - [p2p] \#7035 Remove legacy P2P routing implementation and
    associated configuration options (@tychoish)

- Go API

  - [blocksync] \#7046 Remove v2 implementation of the blocksync
    service and recactor, which was disabled in the previous release
    (@tychoish)
  - [p2p] \#7064 Remove WDRR queue implementation. (@tychoish)

- Blockchain Protocol

### FEATURES

- [mempool, rpc] \#7041  Add removeTx operation to the RPC layer. (@tychoish)

### IMPROVEMENTS

### BUG FIXES
