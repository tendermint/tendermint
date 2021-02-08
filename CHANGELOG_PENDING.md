# Unreleased Changes

## v0.34.4

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

### BUG FIXES

- [light] \#6022 Fix a bug when the number of validators equals 100 (@melekes)
- [light] \#6026 Fix a bug when height isn't provided for the rpc calls: `/commit` and `/validators` (@cmwaters)
- [evidence] \#6068 Terminate broadcastEvidenceRoutine when peer is stopped (@melekes)
