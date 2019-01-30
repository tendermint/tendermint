## v0.30.0

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
- [tools] add go-deadlock tool to help detect deadlocks

### BUG FIXES:
- [node] \#3186 EventBus and indexerService should be started before first block (for replay last block on handshake) execution
- [p2p] \#3232 Fix infinite loop leading to deadlock in addrbook
