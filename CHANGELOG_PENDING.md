## v0.32.2

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API
  - [libs] \#3811 Remove `db` from libs in favor of `https://github.com/tendermint/tm-cmn`

### FEATURES:

- [blockchain] \#3561 Blockchain Reorg Refactor, the new reactor currently sits under a feature flag, see [here](https://github.com/tendermint/tendermint/blob/master/config/toml.go#L297) on how to use it, for further information about the versions please see: [ADR-40](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-040-blockchain-reactor-refactor.md) & [ADR-43](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-043-blockchain-riri-org.md)

### IMPROVEMENTS:

- [abci] \#3809 Recover from application panics in `server/socket_server.go` to allow socket cleanup (@ruseinov)
- [rpc] \#2252 Add `/broadcast_evidence` endpoint to submit double signing and other types of evidence
- [rpc] \#3818 Make `max_body_bytes` and `max_header_bytes` configurable
- [p2p] \#3664 p2p/conn: reuse buffer when write/read from secret connection

### BUG FIXES:
