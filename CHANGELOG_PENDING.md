# Unreleased Changes

## v0.34.11

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

    - [Version] \#6494 `TMCoreSemVer` has been renamed to `TMVersion`.
        - It is not required any longer to set ldflags to set version strings

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [statesync] \#6378 Retry requests for snapshots and add a minimum discovery time (5s) for new snapshots.

### BUG FIXES

- [evidence] \#6375 Fix bug with inconsistent LightClientAttackEvidence hashing (cmwaters)
