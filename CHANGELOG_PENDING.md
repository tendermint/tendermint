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

### IMPROVEMENTS:

- [abci] \#3809 Recover from application panics in `server/socket_server.go` to allow socket cleanup (@ruseinov)
- [rpc] \#3818 Make `max_body_bytes` and `max_header_bytes` configurable
- [p2p] \#3664 p2p/conn: reuse buffer when write/read from secret connection

### BUG FIXES:
