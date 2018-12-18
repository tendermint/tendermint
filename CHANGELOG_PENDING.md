## v0.27.4

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [cli] Removed `node` `--proxy_app=dummy` option. Use `kvstore` (`persistent_kvstore`) instead.
- [cli] Renamed `node` `--proxy_app=nilapp` to `--proxy_app=noop`.

- [privval] \#2926 split up `PubKeyMsg` into `PubKeyRequest` and `PubKeyResponse` to be consistent with other message types

* Apps

* Go API  
- [types] \#2926 memoize consensus public key on initialization of remote signer and return the memoized key on 
`PrivValidator.GetPubKey()` instead of requesting it again 

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
- [types] \#2926 do not panic if retrieving the private validator's public key fails
