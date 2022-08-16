# Unreleased Changes

## v0.34.21

Release highlights include:
- A new RPC configuration flag, `discard_abci_responses`, which, if enabled,
  discards all ABCI responses except the latest one in order to reduce disk
  space usage in the state store. Note that, if enabled, this means that the
  `block_results` RPC endpoint will only be able to return the latest block.
- A CLI command, `reindex-event`, is introduced to re-index block and tx events
  to the event sinks. You can run this command when the event store backend
  dropped/disconnected or you want to replace the backend. Note that, if
  `discard_abci_responses` is enabled, you will not be able to use this command.

See below for more details on specific changes in this release.

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

- [config] \#9054 Flag added to discard all ABCI responses except the last in
  order to save on storage space in the state store.

### BUG FIXES

- [#9103] fix unsafe-reset-all for working with home path (@rootwarp)
