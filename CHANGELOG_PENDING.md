## v0.33.3

- Nodes are no longer guaranteed to contain all blocks up to the latest height. The ABCI app can now control which blocks to retain through the ABCI field `ResponseCommit.retain_height`, all blocks and associated data below this height will be removed.

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

### FEATURES:

- [abci] Add `ResponseCommit.retain_height` field, which will automatically remove blocks below this height.
- [rpc] Add `/status` response fields for the earliest block available on the node
- [rpc] [\#4611](https://github.com/tendermint/tendermint/pull/4611) Add `codespace` to `ResultBroadcastTx` (@whylee259)

### IMPROVEMENTS:

- [blockchain] Add `Base` to blockchain reactor P2P messages `StatusRequest` and `StatusResponse`
- [example/kvstore] Add `RetainBlocks` option to control block retention
- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`
- [Docker] \#4569 Default configuration added to docker image (you can still mount your own config the same way) (@greg-szabo)

### BUG FIXES:

- [rpc] \#4568 Fix panic when `Subscribe` is called, but HTTP client is not running (@melekes)
  `Subscribe`, `Unsubscribe(All)` methods return an error now.
