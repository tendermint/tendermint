# Pending

## v0.26.3

*TBD*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
  - [rpc] \#2791 Functions that start HTTP servers are now blocking:
    - Impacts: StartHTTPServer, StartHTTPAndTLSServer, and StartGRPCServer,
    - These functions now take a `net.Listener` instead of an address

* Blockchain Protocol

* P2P Protocol


### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
