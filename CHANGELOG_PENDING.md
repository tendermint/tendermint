## v0.27.1

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

- [privval] \#2926 split up `PubKeyMsg` into `PubKeyRequest` and `PubKeyResponse` to be consistent with other message types

* Apps

* Go API  
- [types] \#2926 memoize consensus public key on initialization of remote signer and return the memoized key on 
`PrivValidator.GetPubKey()` instead of requesting it again 
- [types] \#2981 Remove `PrivValidator.GetAddress()`

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [rpc] Add `UnconfirmedTxs(limit)` and `NumUnconfirmedTxs()` methods to HTTP/Local clients (@danil-lashin)
- [ci/cd] Updated CircleCI job to trigger website build when docs are updated

### BUG FIXES:
- [kv indexer] \#2912 don't ignore key when executing CONTAINS
- [types] \#2926 do not panic if retrieving the private validator's public key fails
- [mempool] \#2994 Don't allow txs with negative gas wanted
- [p2p] \#2715 fix a bug where seeds don't disconnect from a peer after 3h
