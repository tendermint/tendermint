## v0.27.2

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

- [dep] \#3027 Revert to mainline Go crypto library, eliminating the modified
  `bcrypt.GenerateFromPassword`

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:

- [p2p] [\#3025](https://github.com/tendermint/tendermint/pull/3025) Revert to using defers in addrbook.  Fixes deadlocks in pex and consensus upon invalid ExternalAddr/ListenAddr configuration.
