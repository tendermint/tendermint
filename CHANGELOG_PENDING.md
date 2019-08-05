## v0.32.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

- [privval] \#3370 Refactors and simplifies validator/kms connection handling. Please refer to thttps://github.com/tendermint/tendermint/pull/3370#issue-257360971
- [consensus] \#3839 Reduce "Error attempting to add vote" message severity (Error -> Info)

### BUG FIXES:

- [config] \#3868 move misplaced `max_msg_bytes` into mempool section
