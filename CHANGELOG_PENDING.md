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

<<<<<<< HEAD
=======
- [rpc] [\#7270](https://github.com/tendermint/tendermint/pull/7270) Add `header` and `header_by_hash` RPC Client queries. (@fedekunze)
- [cli] [#7033](https://github.com/tendermint/tendermint/pull/7033) Add a `rollback` command to rollback to the previous tendermint state in the event of non-determinstic app hash or reverting an upgrade.
- [mempool, rpc] \#7041  Add removeTx operation to the RPC layer. (@tychoish)

>>>>>>> 5f57d84dd (rpc: implement header and header_by_hash queries (#7270))
### IMPROVEMENTS

- [\#7338](https://github.com/tendermint/tendermint/pull/7338) pubsub: Performance improvements for the event query API (backport of #7319) (@creachadair)

### BUG FIXES

- [\#7310](https://github.com/tendermint/tendermint/issues/7310) pubsub: Report a non-nil error when shutting down (fixes #7306).
