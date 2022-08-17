# Unreleased Changes

## v0.34.21

Release highlights include:
- A new `[storage]` configuration section and flag `discard_abci_responses`,
  which, if enabled, discards all ABCI responses except the latest one in order
  to reduce disk space usage in the state store. When enabled, the
  `block_results` RPC endpoint can no longer function and will return an error.
- A new CLI command, `reindex-event`, to re-index block and tx events to the
  event sinks. You can run this command when the event store backend
  dropped/disconnected or you want to replace the backend. When
  `discard_abci_responses` is enabled, you will not be able to use this command.

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- [cli] \#9083 Backport command to reindex missed events (@cmwaters)
- [cli] \#9107 Add the `p2p.external-address` argument to set the node P2P external address (@amimart)

### IMPROVEMENTS

- [config] \#9054 `discard_abci_responses` flag added to discard all ABCI
  responses except the last in order to save on storage space in the state
  store (@samricotta)
- [config] \#9275 Move the `discard_abci_responses` flag into a new `[storage]`
  section.

### BUG FIXES

- [mempool] \#9033 Rework lock discipline to mitigate callback deadlocks in the
  priority mempool
- [cli] \#9103 fix unsafe-reset-all for working with home path (@rootwarp)
