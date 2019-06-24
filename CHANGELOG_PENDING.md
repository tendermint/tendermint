## v0.32.0

*June 24, 2019*

Special thanks to external contributors on this release:
@needkane, @SebastianElvis, @andynog, @Yawning, @wooparadog

This release contains breaking changes to our build and release processes, and
to the ABCI, namely:
- Use Go modules instead of dep
- Bring active development to the `master` Github branch
- ABCI Tags are now Events - see
  [docs](https://github.com/tendermint/tendermint/blob/60827f75623b92eff132dc0eff5b49d2025c591e/docs/spec/abci/abci.md#events)

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config
  - [cli] \#3613 Switch from golang/dep to Go Modules to resolve dependencies:
    It is recommended to switch to Go Modules if your project has tendermint as
    a dependency. Read more on Modules here:
    https://github.com/golang/go/wiki/Modules
  - [rpc] \#3616 Improve `/block_results` response format (`results.DeliverTx`
    -> `results.deliver_tx`). See docs for details.

* Apps
  - [abci] \#1859 `ResponseCheckTx`, `ResponseDeliverTx`, `ResponseBeginBlock`,
    and `ResponseEndBlock` now include `Events` instead of `Tags`. Each `Event`
    contains a `type` and a list of `attributes` (list of key-value pairs)
    allowing for inclusion of multiple distinct events in each response.

* Go API
  - [abci] \#3193 Use RequestDeliverTx and RequestCheckTx in the ABCI
    Application interface
  - [libs/db] [\#3632](https://github.com/tendermint/tendermint/pull/3632) Removed deprecated `LevelDBBackend` const
    If you have `db_backend` set to `leveldb` in your config file, please
    change it to `goleveldb` or `cleveldb`.
  - [p2p] \#3521 Remove NewNetAddressStringWithOptionalID

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [abci/examples] \#3659 Change validator update tx format in the `persistent_kvstore` to use base64 for pubkeys instead of hex (@needkane)
- [consensus] \#3656 Exit if SwitchToConsensus fails
- [p2p] \#3666 Add per channel telemetry to improve reactor observability
- [rpc] [\#3686](https://github.com/tendermint/tendermint/pull/3686) `HTTPClient#Call` returns wrapped errors, so a caller could use `errors.Cause` to retrieve an error code. (@wooparadog)
- [rpc] \#3724 RPC now binds to `127.0.0.1` by default instead of `0.0.0.0`

### BUG FIXES:
- [libs/db] \#3717 Fixed the BoltDB backend's Batch.Delete implementation (@Yawning)
- [libs/db] \#3718 Fixed the BoltDB backend's Get and Iterator implementation (@Yawning)
- [node] \#3716 Fix a bug where `nil` is recorded as node's address
- [node] \#3741 Fix profiler blocking the entire node
