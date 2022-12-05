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

### BUG FIXES

