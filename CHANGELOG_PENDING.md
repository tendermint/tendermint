# Unreleased Changes

## v0.37.0

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

  - [config] \#9259 Rename the fastsync section and the fast_sync key blocksync and block_sync respectively

- Apps

  - [abci/counter] \#6684 Delete counter example app
  - [txResults] \#9175 Remove `gas_used` & `gas_wanted` from being merkelized in the lastresulthash in the header
  - [abci] \#5783 Make length delimiter encoding consistent (`uint64`) between ABCI and P2P wire-level protocols
  - [abci] \#9145 Removes Response/Request `SetOption` from ABCI
  - [abci] \#8656 Added cli command for `PrepareProposal`. (@jmalicevic)
  - [abci] \#8901 Added cli command for `ProcessProposal`. (@hvanz)

- P2P Protocol

- Go API

    - [all] \#9144 Change spelling from British English to American (@cmwaters)
        - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)

### BUG FIXES

[docker] \#9073 enable cross platform build using docker buildx
