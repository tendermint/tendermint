# Unreleased Changes

## v0.34.2

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API
  - [libs/os] `EnsureDir` now propagates IO errors and checks the file type (@erikgrinaker)

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

### BUG FIXES

- [evidence] \#5890 Add a buffer to evidence from consensus to avoid broadcasting and proposing evidence before the
height of such an evidence has finished (@cmwaters)
