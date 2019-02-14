## v0.31.0

**

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [libs/common] TrapSignal accepts logger as a first parameter and does not block anymore
  * previously it was dumping "captured ..." msg to os.Stdout
  * TrapSignal should not be responsible for blocking thread of execution

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
- [consensus] \#3297 Flush WAL on stop to prevent data corruption during
  graceful shutdown
