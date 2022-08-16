# Unreleased Changes

## v0.37.0

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

  - [abci/counter] \#6684 Delete counter example app
  - [txResults] \#9175 Remove `gas_used` & `gas_wanted` from being merkelized in the lastresulthash in the header
  - [abci] \#5783 Make length delimiter encoding consistent (`uint64`) between ABCI and P2P wire-level protocols

- P2P Protocol

- Go API

    - [all] \#9144 Change spelling from British English to American (@cmwaters)
        - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

[cli] \#9171 add `--hard` flag to rollback command (and a boolean to the `RollbackState` method). This will rollback
  state and remove the last block. This command can be triggered multiple times. The application must also rollback
  state to the same height. (@tsutsu, @cmwaters)

### BUG FIXES

[docker] \#9073 enable cross platform build using docker buildx
