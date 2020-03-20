## v0.33.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- Nodes are no longer guaranteed to contain all blocks up to the latest height. The block store can now have a truncated history (via `retain_blocks`) such that all blocks up to the block store base will be missing. All blocks between the base and latest height will be present.

- CLI/RPC/Config

- Apps

- P2P Protocol

  - [blockchain] Add `Base` to blockchain reactor messages `tendermint/blockchain/StatusRequest` and `tendermint/blockchain/StatusResponse`

- Go API

### FEATURES:

- [consensus] Add `retain_blocks` config option to automatically prune old blocks

- [rpc] Add `/status` response fields for the earliest block available on the node

### IMPROVEMENTS:

- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)

- [privval] \#4534 Add `error` as a return value on`GetPubKey()`
- [Docker] \#4569 Default configuration added to docker image (you can still mount your own config the same way) (@greg-szabo)

### BUG FIXES:

- [rpc] \#4568 Fix panic when `Subscribe` is called, but HTTP client is not running (@melekes)
  `Subscribe`, `Unsubscribe(All)` methods return an error now.
