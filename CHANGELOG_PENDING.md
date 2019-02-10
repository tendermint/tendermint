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

* \#3291 Make config.ResetTestRootWithChainID() create concurrency-safe test directories.

### BUG FIXES:
- [consensus] \#3297 Flush WAL on stop to prevent data corruption during
  graceful shutdown
