## v0.31.8

**

### BREAKING CHANGES:

* CLI/RPC/Config
- [rpc] \#3616 Improve `/block_results` response format (`results.DeliverTx` ->
  `results.deliver_tx`). See docs for details.

* Apps

* Go API
- [libs/db] Removed deprecated `LevelDBBackend` const
  * If you have `db_backend` set to `leveldb` in your config file, please
    change it to `goleveldb` or `cleveldb`.

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [p2p] \#3666 Add per channel telemtry to improve reactor observability

### BUG FIXES:
