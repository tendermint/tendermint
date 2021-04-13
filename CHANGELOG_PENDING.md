# Unreleased Changes

## v0.34.10

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
- [p2p/node] \#6339 Fix bug with using custom channels (@cmwaters)

=======
- [types] \#5523 Change json naming of `PartSetHeader` within `BlockID` from `parts` to `part_set_header` (@marbar3778)
- [privval] \#5638 Increase read/write timeout to 5s and calculate ping interval based on it (@JoeKash)
- [blockchain/v1] [\#5701](https://github.com/tendermint/tendermint/pull/5701) Handle peers without blocks (@melekes)
- [blockchain/v1] \#5711 Fix deadlock (@melekes)
- [light] \#6346 Correctly handle too high errors to improve client robustness (@cmwaters)
>>>>>>> d4d2b6606... light: handle too high errors correctly (#6346)
