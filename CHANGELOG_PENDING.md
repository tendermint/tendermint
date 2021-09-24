# Unreleased Changes

## vX.X

Special thanks to external contributors on this release:

Friendly reminder: We have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

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

### IMPROVEMENTS

### BUG FIXES

