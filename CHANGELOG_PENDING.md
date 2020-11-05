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

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [statesync] \#5516 Check that all heights necessary to rebuild state for a snapshot exist before adding the snapshot to the pool. (@erikgrinaker)

### BUG FIXES

- [blockchain/v2] \#5499 Fix "duplicate block enqueued by processor" panic (@melekes)
- [abci/grpc] \#5520 Return async responses in order, to avoid mempool panics. (@erikgrinaker)
- [types] \#5523 Change json naming of `PartSetHeader` within `BlockID` from `parts` to `part_set_header` (@marbar3778)
- [blockchain/v2] \#5530 Fix "processed height 4541 but expected height 4540" panic (@melekes)
- [consensus/wal] Fix WAL autorepair by opening target WAL in read/write mode (@erikgrinaker)
- [blockchain/v2] \#5553 Make the removal of an already removed peer a noop (@melekes)
- [evidence] \#5574 Fix bug where node sends committed evidence to peer (@cmwaters)
- [evidence] \5610 Make it possible for abci evidence to be formed from tm evidence (@cmwaters)
