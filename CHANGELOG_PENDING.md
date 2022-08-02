# Unreleased Changes

## v0.37.0

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

    - [all] [#9144] Change spelling from British English to American (@cmwaters)
        - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub

- Blockchain Protocol

### FEATURES

- [#9083] backport cli command to reindex missed events (@cmwaters)
- [cli] \#9107 Add the `p2p.external-address` argument to set the node P2P external address (@amimart)

### IMPROVEMENTS

### BUG FIXES
