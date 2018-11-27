## v0.27.1

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
- [rpc] \#2408 `/broadcast_tx_commit`: Fix "interface conversion: interface {} in nil, not EventDataTx" panic (could happen if somebody sent a tx using /broadcast_tx_commit while Tendermint was being stopped)
- [kv indexer] \#2912 don't ignore key when executing CONTAINS
