# Unreleased Changes

## vX.X

Special thanks to external contributors on this release:

Friendly reminder: We have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps
  - [proto/tendermint] \#6976 Remove core protobuf files in favor of only housing them in the [tendermint/spec](https://github.com/tendermint/spec) repository.

- P2P Protocol

- Go API

  - [crypto/armor]: \#6963 remove package which is unused, and based on
    deprecated fundamentals. Downstream users should maintain this
    library. (@tychoish)
  - [state] [store] [proxy] [rpc/core]: \#6937 move packages to
    `internal` to prevent consumption of these internal APIs by
    external users. (@tychoish)

- Blockchain Protocol

### FEATURES

- [\#6982](https://github.com/tendermint/tendermint/pull/6982) tendermint binary has built-in suppport for running the e2e application (with state sync support) (@cmwaters).

### IMPROVEMENTS

### BUG FIXES
