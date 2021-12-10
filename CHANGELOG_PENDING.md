# Unreleased Changes

Friendly reminder: We have a [bug bounty program](https://hackerone.com/cosmos).

## vX.X

Month, DD, YYYY

Special thanks to external contributors on this release:

### BREAKING CHANGES

- CLI/RPC/Config

  - [config] [\#7276](https://github.com/tendermint/tendermint/pull/7276) rpc: Add experimental config params to allow for subscription buffer size control (@thanethomson).

- Apps

- P2P Protocol

  - [p2p] [\#7265](https://github.com/tendermint/tendermint/pull/7265) Peer manager reduces peer score for each failed dial attempts for peers that have not successfully dialed. (@tychoish)

- Go API

- Blockchain Protocol

### FEATURES

- [rpc] [\#7270](https://github.com/tendermint/tendermint/pull/7270) Add `header` and `header_by_hash` RPC Client queries. (@fedekunze) (@cmwaters)

### IMPROVEMENTS
- [internal/protoio] \#7325 Optimized `MarshalDelimited` by inlining the common case and using a `sync.Pool` in the worst case. (@odeke-em)

- [\#7338](https://github.com/tendermint/tendermint/pull/7338) pubsub: Performance improvements for the event query API (backport of #7319) (@creachadair)

### BUG FIXES

- [\#7310](https://github.com/tendermint/tendermint/issues/7310) pubsub: Report a non-nil error when shutting down (fixes #7306).
