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
  - [abci/client, proxy] \#5673 `Async` funcs return an error, `Sync` and `Async` funcs accept `context.Context` (@melekes)
  - [p2p] Removed unused function `MakePoWTarget`. (@erikgrinaker)

- [libs/os] Kill() and {Must,}{Read,Write}File() functions have been removed. (@alessio)

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [crypto/ed25519] \#5632 Adopt zip215 `ed25519` verification. (@marbar3778)
- [privval] \#5603 Add `--key` to `init`, `gen_validator`, `testnet` & `unsafe_reset_priv_validator` for use in generating `secp256k1` keys.
- [abci/client] \#5673 `Async` requests return an error if queue is full (@melekes)
- [mempool] \#5673 Cancel `CheckTx` requests if RPC client disconnects or times out (@melekes)
- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)
- [libs/bits] \#5720 Copy elems and validate BitArray in FromProto (@melekes)

### BUG FIXES

- [types] \#5523 Change json naming of `PartSetHeader` within `BlockID` from `parts` to `part_set_header` (@marbar3778)
- [privval] \#5638 Increase read/write timeout to 5s and calculate ping interval based on it (@JoeKash)
- [blockchain/v1] [\#5701](https://github.com/tendermint/tendermint/pull/5701) Handle peers without blocks (@melekes)
- [crypto] \#5707 Fix infinite recursion in string formatting of Secp256k1 keys (@erikgrinaker)
