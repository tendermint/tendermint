# Pending

## v0.26.3

*November 17th, 2018*

Special thanks to external contributors on this release:
@danil-lashin, @kevlubkcm, @krhubert, @srmo

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* Go API
  - [rpc] [\#2791](https://github.com/tendermint/tendermint/issues/2791) Functions that start HTTP servers are now blocking:
    - Impacts `StartHTTPServer`, `StartHTTPAndTLSServer`, and `StartGRPCServer`
    - These functions now take a `net.Listener` instead of an address
  - [rpc] [\#2767](https://github.com/tendermint/tendermint/issues/2767) Subscribing to events
  `NewRound` and `CompleteProposal` return new types `EventDataNewRound` and
  `EventDataCompleteProposal`, respectively, instead of the generic `EventDataRoundState`. (@kevlubkcm)

### FEATURES:

- [log] [\#2843](https://github.com/tendermint/tendermint/issues/2843) New `log_format` config option, which can be set to 'plain' for colored
  text or 'json' for JSON output
- [types] [\#2767](https://github.com/tendermint/tendermint/issues/2767) New event types EventDataNewRound (with ProposerInfo) and EventDataCompleteProposal (with BlockID). (@kevlubkcm)

### IMPROVEMENTS:

- [dep] [\#2844](https://github.com/tendermint/tendermint/issues/2844) Dependencies are no longer pinned to an exact version in the
  Gopkg.toml:
  - Serialization libs are allowed to vary by patch release
  - Other libs are allowed to vary by minor release
- [p2p] [\#2857](https://github.com/tendermint/tendermint/issues/2857) "Send failed" is logged at debug level instead of error.
- [rpc] [\#2780](https://github.com/tendermint/tendermint/issues/2780) Add read and write timeouts to HTTP servers
- [state] [\#2848](https://github.com/tendermint/tendermint/issues/2848) Make "Update to validators" msg value pretty (@danil-lashin)

### BUG FIXES:
- [consensus] [\#2819](https://github.com/tendermint/tendermint/issues/2819) Don't send proposalHearbeat if not a validator
- [docs] [\#2859](https://github.com/tendermint/tendermint/issues/2859) Fix ConsensusParams details in spec
- [libs/autofile] [\#2760](https://github.com/tendermint/tendermint/issues/2760) Comment out autofile permissions check - should fix
  running Tendermint on Windows
- [p2p] [\#2869](https://github.com/tendermint/tendermint/issues/2869) Set connection config properly instead of always using default
- [p2p/pex] [\#2802](https://github.com/tendermint/tendermint/issues/2802) Seed mode fixes:
  - Only disconnect from inbound peers
  - Use FlushStop instead of Sleep to ensure all messages are sent before
    disconnecting
