## v0.31.8

**

### BREAKING CHANGES:

* CLI/RPC/Config
  * [rpc] \#3616 Improve `/block_results` response format (`results.DeliverTx` ->
  `results.deliver_tx`). See docs for details.

* Apps
  * [abci] \#1859 `ResponseCheckTx`, `ResponseDeliverTx`, `ResponseBeginBlock`,
  and `ResponseEndBlock` now include `Events` instead of `Tags`. Each `Event`
  contains a `type` and a list of `attributes` (list of key-value pairs) allowing
  for inclusion of multiple distinct events in each response.

* Go API
  * [libs/db] Removed deprecated `LevelDBBackend` const
    * If you have `db_backend` set to `leveldb` in your config file, please
    change it to `goleveldb` or `cleveldb`.

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [p2p] \#3666 Add per channel telemtry to improve reactor observability

* [rpc] [\#3686](https://github.com/tendermint/tendermint/pull/3686) `HTTPClient#Call` returns wrapped errors, so a caller could use `errors.Cause` to retrieve an error code.

### BUG FIXES:
