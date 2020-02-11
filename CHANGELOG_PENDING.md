## v0.33.1

\*\*

Special thanks to external contributors on this release:
@princesinha19

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:

- [rpc] [\#3333] Add `order_by` to `/tx_search` endpoint, allowing to change default ordering from asc to desc (more in the future) (@princesinha19)

### IMPROVEMENTS:

- [proto] [\#4369] Add [buf](https://buf.build/) for usage with linting and checking if there are breaking changes with the master branch.
- [proto] [\#4369] Add `make proto-gen` cmd to generate proto stubs outside of GOPATH.


### BUG FIXES:

- [node] [#\4311] Use `GRPCMaxOpenConnections` when creating the gRPC server, not `MaxOpenConnections`
- [rpc] [#\4319] Check `BlockMeta` is not nil in `/block` & `/block_by_hash`
