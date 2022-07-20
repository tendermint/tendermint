# Unreleased Changes

Friendly reminder: We have a [bug bounty program](https://hackerone.com/cosmos).

## v0.35.9

Month DD, YYYY

Special thanks to external contributors on this release:

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- fix: assignment copies lock value in `BitArray.UnmarshalJSON()` (@lklimek)
- [light] \#7640 Light Client: fix absence proof verification (@ashcherbakov)
- [light] \#7641 Light Client: fix querying against the latest height (@ashcherbakov)
- [cli] [#7837](https://github.com/tendermint/tendermint/pull/7837) fix app hash in state rollback. (@yihuang)
- [cli] \#8276 scmigrate: ensure target key is correctly renamed. (@creachadair)
- [cli] \#8294 keymigrate: ensure block hash keys are correctly translated. (@creachadair)
- [cli] \#8352 keymigrate: ensure transaction hash keys are correctly translated. (@creachadair)
- (indexer) \#8625 Fix overriding tx index of duplicated txs.
- [mempool] \#8944 Fix unbounded heap growth in the priority mempool. (@creachadair)

### BUG FIXES
