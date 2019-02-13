## v0.31.0

**

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [consensus] \#3300 WAL interface now requires a `Flush` method

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
- [consensus] \#3297 Flush WAL on stop to prevent data corruption during
  graceful shutdown
- [consensus] \#3300 Flush WAL periodically and prior to signing votes/proposals
  to help prevent data corruption
