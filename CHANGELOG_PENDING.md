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
  - [abci] \#9145 Removes unused Response/Request `SetOption` from ABCI (@samricotta)
  - [abci] \#8656 Added cli command for `PrepareProposal`. (@jmalicevic)
  - [abci] \#8901 Added cli command for `ProcessProposal`. (@hvanz)
  - [abci/params] \#9287 Deduplicate `ConsensusParams` and `BlockParams` so only `types` proto definitions are used (@cmwaters)
    - Remove `TimeIotaMs` and use a hard-coded 1 millisecond value to ensure monotonically increasing block times.
    - Rename `AppVersion` to `App` so as to not stutter.

- P2P Protocol

- Go API

    - [all] \#9144 Change spelling from British English to American (@cmwaters)
        - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)

### BUG FIXES

- [consensus] \#9229 fix round number of `enterPropose` when handling `RoundStepNewRound` timeout. (@fatcat22)
- [docker] \#9073 enable cross platform build using docker buildx
