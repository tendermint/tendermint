# Unreleased Changes

## vX.X

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config
  - [config] \#5598 The `test_fuzz` and `test_fuzz_config` P2P settings have been removed. (@erikgrinaker)

- Apps

- P2P Protocol

- Go API
  - [p2p] Removed unused function `MakePoWTarget`. (@erikgrinaker)

- [libs/os] Kill() and {Must,}{Read,Write}File() functions have been removed. (@alessio)

- Blockchain Protocol

### FEATURES



### IMPROVEMENTS

- [crypto/ed25519] \#5632 Adopt zip215 `ed25519` verification. (@marbar3778)
- [privval] \#5603 Add `--key` to `init`, `gen_validator`, `testnet` & `unsafe_reset_priv_validator` for use in generating `secp256k1` keys.

### BUG FIXES

- [types] \#5523 Change json naming of `PartSetHeader` within `BlockID` from `parts` to `part_set_header` (@marbar3778)
- [privval] \#5638 Increase read/write timeout to 5s and calculate ping interval based on it (@JoeKash)
