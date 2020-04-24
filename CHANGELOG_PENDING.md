## v0.33.5

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

### FEATURES:

- [evidence] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Handle evidence from light clients (@melekes)
- [lite2] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Submit conflicting headers, if any, to a full node & all witnesses (@melekes)

### IMPROVEMENTS:

- [abci/server] [\#4719](https://github.com/tendermint/tendermint/pull/4719) Print panic & stack trace to STDERR if logger is not set (@melekes)
- [types] [\#4638](https://github.com/tendermint/tendermint/pull/4638) Implement `Header#ValidateBasic` (@alexanderbez)
- [txindex] [\#4466](https://github.com/tendermint/tendermint/pull/4466) Allow to index an event at runtime (@favadi)

### BUG FIXES:
