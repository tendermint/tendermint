## v0.33.4

- Nodes are no longer guaranteed to contain all blocks up to the latest height. The ABCI app can now control which blocks to retain through the ABCI field `ResponseCommit.retain_height`, all blocks and associated data below this height will be removed.

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

  - [rpc/client] [\#4628](https://github.com/tendermint/tendermint/pull/4628) Split out HTTP and local clients into `http` and `local` packages (@erikgrinaker).
  - [lite2] [\#4616](https://github.com/tendermint/tendermint/pull/4616) Make `maxClockDrift` an option (@melekes).
    `Verify/VerifyAdjacent/VerifyNonAdjacent` now accept `maxClockDrift time.Duration`.

### FEATURES:

- [abci] Add `ResponseCommit.retain_height` field, which will automatically remove blocks below this height. This bumps the ABCI version to 0.16.2.
- [rpc] Add `/status` response fields for the earliest block available on the node
- [rpc] [\#4611](https://github.com/tendermint/tendermint/pull/4611) Add `codespace` to `ResultBroadcastTx` (@whylee259)
- [cmd] [\#4665](https://github.com/tendermint/tendermint/pull/4665) New `tedermint completion` command to generate Bash/Zsh completion scripts (@alessio).

### IMPROVEMENTS:

- [blockchain] Add `Base` to blockchain reactor P2P messages `StatusRequest` and `StatusResponse`
- [example/kvstore] Add `RetainBlocks` option to control block retention
- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`
- [Docker] \#4569 Default configuration added to docker image (you can still mount your own config the same way) (@greg-szabo)
- [lite2] [\#4562](https://github.com/tendermint/tendermint/pull/4562) Cache headers when using bisection (@cmwaters)
- [all] [\#4608](https://github.com/tendermint/tendermint/pull/4608) Give reactors descriptive names when they're initialized
- [lite2] [\#4575](https://github.com/tendermint/tendermint/pull/4575) Use bisection for within-range verification (@cmwaters)
- [tools] \#4615 Allow developers to use Docker to generate proto stubs, via `make proto-gen-docker`.
- [p2p] \#4621(https://github.com/tendermint/tendermint/pull/4621) ban peers when messages are unsolicited or too frequent (@cmwaters)
- [evidence] [\#4632](https://github.com/tendermint/tendermint/pull/4632) Inbound evidence checked if already existing (@cmwaters)
- [rpc] [\#4703](https://github.com/tendermint/tendermint/pull/4703) Add `count` and `total` to `/validators` response (@melekes)

### BUG FIXES:

- [rpc] \#4568 Fix panic when `Subscribe` is called, but HTTP client is not running (@melekes)
  `Subscribe`, `Unsubscribe(All)` methods return an error now.
