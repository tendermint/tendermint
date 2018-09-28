# Pending

Special thanks to external contributors on this release:

BREAKING CHANGES:

* CLI/RPC/Config
- [config] `mempool.wal` is disabled by default

* Apps

* Go API
- [node] Remove node.RunForever
- [config] \#2232 timeouts as time.Duration, not ints

* Blockchain Protocol
  * [types] \#2459 `Vote`/`Proposal`/`Heartbeat` use amino encoding instead of JSON in `SignBytes`.
  * [privval] \#2459 Split `SocketPVMsg`s implementations into Request and Response, where the Response may contain a error message (returned by the remote signer). 

* P2P Protocol

FEATURES:

IMPROVEMENTS:
- [consensus] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics
- [p2p] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics
- [config] \#2232 added ValidateBasic method, which performs basic checks

BUG FIXES:
- [autofile] \#2428 Group.RotateFile need call Flush() before rename (@goolAdapter)
- [node] \#2434 Make node respond to signal interrupts while sleeping for genesis time
