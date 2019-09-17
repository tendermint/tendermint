## v0.32.4

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

- [rpc] \#2010 Add NewHTTPWithClient and NewJSONRPCClientWithHTTPClient (note these and NewHTTP, NewJSONRPCClient functions panic if remote is invalid) (@gracenoah)
- [rpc] \#3984 Add `MempoolClient` interface to `Client` interface
- [rpc] \#3471 Paginate `/validator` response, default returns all validators with no limit

### BUG FIXES:

- [consensus] \#3908 Wait `timeout_commit` to pass even if `create_empty_blocks` is `false`
- [mempool] \#3968 Fix memory loading error on 32-bit machines (@jon-certik)
