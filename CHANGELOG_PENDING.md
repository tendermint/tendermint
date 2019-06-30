## v0.32.1

**

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
  - [libs] Remove unused `db/debugDB` and `common/colors.go` & `errors/errors.go` files (@marbar3778)


* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
- [p2p] \#3338 Prevent "sent next PEX request too soon" errors by not calling
  ensurePeers outside of ensurePeersRoutine
