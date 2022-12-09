# Unreleased Changes

## v0.34.25

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- `[rpc]` [\#9759] Added `match_event` query parameter to indicate to Tendermint that the query should match event attributes within events, not only within a height.

### IMPROVEMENTS

- `[state/kvindexer]` [\#9759] Added `match.event` keyword to support condition evalution based on the event attributes belong to. (@jmalicevic)
- [crypto] \#9250 Update to use btcec v2 and the latest btcutil. (@wcsiu)
- [consensus] \#9760 Save peer LastCommit correctly to achieve 50% reduction in gossiped precommits. (@williambanfield)
- [metrics] \#9733 Add metrics for timing the consensus steps and for the progress of block gossip. (@williambanfield)

### BUG FIXES

