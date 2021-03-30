# Unreleased Changes

## v0.34.9

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API
    - [rpc/jsonrpc/server] \#6204 Modify `WriteRPCResponseHTTP(Error)` to return an error (@melekes)

- Blockchain Protocol

### FEATURES

- [rpc] \#6226 Index block events and expose a new RPC method, `/block_search`, to allow querying for blocks by `BeginBlock` and `EndBlock` events. (@alexanderbez)

### IMPROVEMENTS

### BUG FIXES

- [rpc/jsonrpc/server] \#6191 Correctly unmarshal `RPCRequest` when data is `null` (@melekes)
- [p2p] \#6289 Fix "unknown channels" bug on CustomReactors (@gchaincl)
