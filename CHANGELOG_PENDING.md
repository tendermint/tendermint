## v0.32.4

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:
- [abci] [\#3919](https://github.com/tendermint/tendermint/issues/3919)`CheckTxType` takes values of `CheckTxType_Local` , `CheckTxType_Remote` and `CheckTxType_Recheck`, indicating whether this is a local or remote tx being checked or whether this tx is being rechecked after a block commit. This allows applications to skip certain expensive operations, like signature checking, if they've already been done once.

### IMPROVEMENTS:

### BUG FIXES:
