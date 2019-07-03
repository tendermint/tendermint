## v0.31.8

**

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
  - [libs/db] [\#3632](https://github.com/tendermint/tendermint/pull/3632) Removed deprecated `LevelDBBackend` const
    If you have `db_backend` set to `leveldb` in your config file, please
    change it to `goleveldb` or `cleveldb`.
  - [p2p] \#3521 Remove NewNetAddressStringWithOptionalID

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [node] Refactor `NewNode` to use functional options to make it more flexible
  and extensible in the future.
- [node] [\#3730](https://github.com/tendermint/tendermint/pull/3730) Add `CustomReactors` option to `NewNode` allowing caller to pass
  custom reactors to run inside Tendermint node (@ParthDesai)

### IMPROVEMENTS:
- [p2p] \#3666 Add per channel telemetry to improve reactor observability
- [rpc] [\#3686](https://github.com/tendermint/tendermint/pull/3686) `HTTPClient#Call` returns wrapped errors, so a caller could use `errors.Cause` to retrieve an error code. (@wooparadog)

### BUG FIXES:
- [libs/db] \#3717 Fixed the BoltDB backend's Batch.Delete implementation (@Yawning)
- [libs/db] \#3718 Fixed the BoltDB backend's Get and Iterator implementation (@Yawning)
