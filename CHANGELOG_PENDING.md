# Unreleased Changes

## v0.37.0

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

  - [abci/counter] \#6684 Delete counter example app

- P2P Protocol

- Go API

    - [all] [#9144] Change spelling from British English to American (@cmwaters)
        - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)

### BUG FIXES
