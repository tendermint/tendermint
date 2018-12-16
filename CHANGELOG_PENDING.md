## v0.27.2

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

- [node] \#3025 Validate NodeInfo addresses on startup.

### BUG FIXES:

- [p2p] [\#3025](https://github.com/tendermint/tendermint/pull/3025) Revert to using defers in addrbook.  Fixes deadlocks in pex and consensus upon invalid ExternalAddr/ListenAddr configuration.
