# Unreleased Changes

## v0.34.5

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

<<<<<<< HEAD
=======
- [ABCI] \#6124 Fixes a panic condition during callback execution in `ReCheckTx` during high tx load. (@alexanderbez)
- [types] \#5523 Change json naming of `PartSetHeader` within `BlockID` from `parts` to `part_set_header` (@marbar3778)
- [privval] \#5638 Increase read/write timeout to 5s and calculate ping interval based on it (@JoeKash)
- [blockchain/v1] [\#5701](https://github.com/tendermint/tendermint/pull/5701) Handle peers without blocks (@melekes)
- [blockchain/v1] \#5711 Fix deadlock (@melekes)
>>>>>>> 5b52f8778... ABCI: Fix ReCheckTx for Socket Client (#6124)
