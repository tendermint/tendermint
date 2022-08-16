# Unreleased Changes

## v0.34.21

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- [#9083] backport cli command to reindex missed events (@cmwaters)
- [cli] \#9107 Add the `p2p.external-address` argument to set the node P2P external address (@amimart)

### IMPROVEMENTS

- [config] \#9054 Flag added to overwrite abciresponses.

### BUG FIXES

- [#9103] fix unsafe-reset-all for working with home path (@rootwarp)
