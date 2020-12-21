# Unreleased Changes

## v0.34.1

Special thanks to external contributors on this release:

@p4u from vocdoni.io reported that the mempool might behave incorrectly under a
high load. The consequences can range from pauses between blocks to the peers
disconnecting from this node. As a temporary remedy (until the mempool package
is refactored), the `max-batch-bytes` was disabled. Transactions will be sent
one by one without batching.

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

  - [cli] \#5786 deprecate snake_case commands for hyphen-case (@cmwaters)

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [mempool] \#5813 Add `keep-invalid-txs-in-cache` config option. When set to true, mempool will keep invalid transactions in the cache (@p4u)

### BUG FIXES

- [crypto] \#5707 Fix infinite recursion in string formatting of Secp256k1 keys (@erikgrinaker)
- [mempool] \#5800 Disable `max-batch-bytes` (@melekes)
