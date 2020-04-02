## v0.33.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

  - [rpc/client] [\#4628](https://github.com/tendermint/tendermint/pull/4628) Split out HTTP and local clients into `http` and `local` packages (@erikgrinaker).
  - [lite2] [\#4616](https://github.com/tendermint/tendermint/pull/4616) Make `maxClockDrift` an option (@melekes).
    `Verify/VerifyAdjacent/VerifyNonAdjacent` now accept `maxClockDrift time.Duration`.

### FEATURES:

- [rpc] [\#4611](https://github.com/tendermint/tendermint/pull/4611) Add `codespace` to `ResultBroadcastTx` (@whylee259)

### IMPROVEMENTS:

- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`
- [Docker] \#4569 Default configuration added to docker image (you can still mount your own config the same way) (@greg-szabo)
- [lite2] [\#4562](https://github.com/tendermint/tendermint/pull/4562) Cache headers when using bisection (@cmwaters)
- [all] [\#4608](https://github.com/tendermint/tendermint/pull/4608) Give reactors descriptive names when they're initialized
- [lite2] [\#4575](https://github.com/tendermint/tendermint/pull/4575) Use bisection for within-range verification (@cmwaters)
- [tools] \#4615 Allow developers to use Docker to generate proto stubs, via `make proto-gen-docker`.

### BUG FIXES:

- [rpc] \#4568 Fix panic when `Subscribe` is called, but HTTP client is not running (@melekes)
  `Subscribe`, `Unsubscribe(All)` methods return an error now.
