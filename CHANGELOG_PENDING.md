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

- [log] \#2843 New `log_format` config option, which can be set to 'plain' for colored
  text or 'json' for JSON output

- [types] \#2767 New event types EventDataNewRound (with ProposerInfo) and EventDataCompleteProposal (with BlockID). (@kevlubkcm)

### IMPROVEMENTS:

- [state] \#2765 Make "Update to validators" msg value pretty (@danil-lashin)
- [p2p] \#2857 "Send failed" is logged at debug level instead of error.

### BUG FIXES:
