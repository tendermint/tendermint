## v0.28.0

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [cli] Removed `node` `--proxy_app=dummy` option. Use `kvstore` (`persistent_kvstore`) instead.
- [cli] Renamed `node` `--proxy_app=nilapp` to `--proxy_app=noop`.
- [config] \#2992 `allow_duplicate_ip` is now set to false
- [privval] \#2926 split up `PubKeyMsg` into `PubKeyRequest` and `PubKeyResponse` to be consistent with other message types
- [privval] \#2923 listen for unix socket connections instead of dialing them

* Apps

* Go API
- [types] \#2926 memoize consensus public key on initialization of remote signer and return the memoized key on
`PrivValidator.GetPubKey()` instead of requesting it again
- [types] \#2981 Remove `PrivValidator.GetAddress()`

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [privval] \#1181 Split immutable and mutable parts of `priv_validator.json`

### IMPROVEMENTS:
- [p2p/conn] \#3111 make SecretConnection thread safe
- [privval] \#2923 retry RemoteSigner connections on error
- [rpc] \#3047 Include peer's remote IP in `/net_info`

### BUG FIXES:
- [types] \#2926 do not panic if retrieving the private validator's public key fails
- [crypto/multisig] \#3102 fix multisig keys address length
- [crypto/encoding] \#3101 Fix `PubKeyMultisigThreshold` unmarshalling into `crypto.PubKey` interface
