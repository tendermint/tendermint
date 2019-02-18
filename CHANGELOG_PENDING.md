## v0.31.0

**

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

- [config] \#3291 Make config.ResetTestRootWithChainID() create concurrency-safe test directories.

### BUG FIXES:

* [consensus] \#3297 Flush WAL on stop to prevent data corruption during
  graceful shutdown
- [consensus] \#3302 Reset TriggeredTimeoutPrecommit before starting next
  height
- [rpc] \#3251 Fix /net_info#peers#remote_ip format. New format spec:
  * dotted decimal ("192.0.2.1"), if ip is an IPv4 or IP4-mapped IPv6 address
  * IPv6 ("2001:db8::1"), if ip is a valid IPv6 address
* [cmd] \#3314 Return an error on `show_validator` when the private validator
  file does not exist
* [p2p] \#3321 Authenticate a peer against its NetAddress.ID while dialing 
