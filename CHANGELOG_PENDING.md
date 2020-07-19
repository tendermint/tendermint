## v0.32.2

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API
  - [libs] \#3811 Remove `db` from libs in favor of `https://github.com/tendermint/tm-db`

### FEATURES:

- [node] Allow replacing existing p2p.Reactor(s) using [`CustomReactors`
  option](https://godoc.org/github.com/tendermint/tendermint/node#CustomReactors).
  Warning: beware of accidental name clashes. Here is the list of existing
  reactors: MEMPOOL, BLOCKCHAIN, CONSENSUS, EVIDENCE, PEX.

### IMPROVEMENTS:

- [p2p] \#3834 Do not write 'Couldn't connect to any seeds' error log if there are no seeds in config file
- [abci] \#3809 Recover from application panics in `server/socket_server.go` to allow socket cleanup (@ruseinov)
- [rpc] \#2252 Add `/broadcast_evidence` endpoint to submit double signing and other types of evidence
- [rpc] \#3818 Make `max_body_bytes` and `max_header_bytes` configurable
- [p2p] \#3664 p2p/conn: reuse buffer when write/read from secret connection
- [mempool] \#3826 Make `max_msg_bytes` configurable
- [blockchain] \#3561 Add early version of the new blockchain reactor, which is supposed to be more modular and testable compared to the old version. To try it, you'll have to change `version` in the config file, [here](https://github.com/tendermint/tendermint/blob/master/config/toml.go#L303) NOTE: It's not ready for a production yet. For further information, see [ADR-40](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-040-blockchain-reactor-refactor.md) & [ADR-43](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-043-blockchain-riri-org.md)
- [rpc] \#3076 Improve transaction search performance

### BUG FIXES:

- [p2p][\#3644](https://github.com/tendermint/tendermint/pull/3644) Fix error logging for connection stop (@defunctzombie)
- [p2p][\#5136](https://github.com/tendermint/tendermint/pull/5136) Fix error for peer with the same ID but different IPs (@valardragon)
- [rpc] \#3813 Return err if page is incorrect (less than 0 or greater than total pages)