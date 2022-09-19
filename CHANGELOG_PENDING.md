# Unreleased Changes

## v0.38.0

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [pubsub] \#7319 Performance improvements for the event query API (@creachadair)
- [crypto] \#6120 Implement batch verification interface for ed25519 and sr25519. (@marbar3778, @jayt106)
- [types] \#6120 use batch verification for verifying commits signatures.
If the key type supports the batch verification API it will try to batch verify. If the verification fails we will single verify each signature. (@marbar3778)

### BUG FIXES

## v0.37.0

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config
  - [config] \#9259 Rename the fastsync section and the fast_sync key blocksync and block_sync respectively

- Apps
  - [abci/counter] \#6684 Delete counter example app
  - [abci] \#5783 Make length delimiter encoding consistent (`uint64`) between ABCI and P2P wire-level protocols
  - [abci] \#9145 Removes unused Response/Request `SetOption` from ABCI (@samricotta)
  - [abci/params] \#9287 Deduplicate `ConsensusParams` and `BlockParams` so only `types` proto definitions are used (@cmwaters)
    - Remove `TimeIotaMs` and use a hard-coded 1 millisecond value to ensure monotonically increasing block times.
    - Rename `AppVersion` to `App` so as to not stutter.
  - [types] \#9287 Reduce the use of protobuf types in core logic. (@cmwaters)
    - `ConsensusParams`, `BlockParams`, `ValidatorParams`, `EvidenceParams`, `VersionParams` have become native types.
      They still utilize protobuf when being sent over the wire or written to disk.
    - Moved `ValidateConsensusParams` inside (now native type) `ConsensusParams`, and renamed it to `ValidateBasic`.
  - [abci] \#9301 New ABCI methods `PrepareProposal` and `ProcessProposal` which give the app control over transactions proposed and allows for verification of proposed blocks.
  - [abci] \#8216 Renamed `EvidenceType` to `MisbehaviorType` and `Evidence` to `Misbehavior` as a more accurate label of their contents. (@williambanfield, @sergio-mena)
  - [abci] \#9122 Renamed `LastCommitInfo` to `CommitInfo` in preparation for vote extensions. (@cmwaters)
  - [abci] \#8656, \#8901 Added cli commands for `PrepareProposal` and `ProcessProposal`. (@jmalicevic, @hvanz)
  - [abci] \#6403 Change the `key` and `value` fields from `[]byte` to `string` in the `EventAttribute` type. (@alexanderbez)

- P2P Protocol

- Go API
    - [all] \#9144 Change spelling from British English to American (@cmwaters)
        - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub

- Blockchain Protocol

### FEATURES

- [abci] \#9301 New ABCI methods `PrepareProposal` and `ProcessProposal` which give the app control over transactions proposed and allows for verification of proposed blocks.

### IMPROVEMENTS
- [crypto] \#9250 Update to use btcec v2 and the latest btcutil. (@wcsiu)

- [proto] \#9356 Migrate from `gogo/protobuf` to `cosmos/gogoproto` (@julienrbrt)
- [rpc] \#9276 Added `header` and `header_by_hash` queries to the RPC client (@samricotta)
- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)

### BUG FIXES

- [consensus] \#9229 fix round number of `enterPropose` when handling `RoundStepNewRound` timeout. (@fatcat22)
- [docker] \#9073 enable cross platform build using docker buildx
