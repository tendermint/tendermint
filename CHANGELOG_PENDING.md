## v0.33.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- Nodes are no longer guaranteed to contain all blocks up to the latest height. The block store can now have a truncated history (via `retain_blocks`) such that all blocks before the block store base will be missing.

- CLI/RPC/Config

- Apps

- P2P Protocol

  - [blockchain] Add `Base` to blockchain reactor messages `tendermint/blockchain/StatusRequest` and `tendermint/blockchain/StatusResponse`

- Go API

### FEATURES:

- [consensus] Add `retain_blocks` config option to automatically prune old blocks

- [rpc] Add `/status` fields for the earliest block available on the node

### IMPROVEMENTS:

- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`

### BUG FIXES:
