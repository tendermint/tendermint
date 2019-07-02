## v0.31.8

**

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [abci] \#3809 Recover from application panics in `server/socket_server.go` to allow socket cleanup (@ruseinov)

### BUG FIXES:
- [p2p] \#3338 Prevent "sent next PEX request too soon" errors by not calling
  ensurePeers outside of ensurePeersRoutine
