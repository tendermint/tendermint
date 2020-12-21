# Unreleased Changes

## v0.34.1

Special thanks to external contributors on this release:

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
