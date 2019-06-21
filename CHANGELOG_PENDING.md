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
  - [abci] \#3193 Use RequestDeliverTx and RequestCheckTx in the ABCI interface
  - [abci] \#2127 ABCI / mempool: Add a "Recheck Tx" indicator. Breaks the ABCI
    client interface (`abcicli.Client`) to allow for supplying the ABCI
    `types.RequestCheckTx` and `types.RequestDeliverTx` structs, and lets the
    mempool indicate to the ABCI app whether a CheckTx request is a recheck or
    not.

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [p2p] \#3666 Add per channel telemetry to improve reactor observability
- [rpc] [\#3686](https://github.com/tendermint/tendermint/pull/3686) `HTTPClient#Call` returns wrapped errors, so a caller could use `errors.Cause` to retrieve an error code. (@wooparadog)
- [abci/examples] \#3659 Change validator update tx format (incl. expected pubkey format, which is base64 now) (@needkane)

### BUG FIXES:
- [libs/db] \#3717 Fixed the BoltDB backend's Batch.Delete implementation (@Yawning)
- [libs/db] \#3718 Fixed the BoltDB backend's Get and Iterator implementation (@Yawning)
- [node] \#3716 Fix a bug where `nil` is recorded as node's address
