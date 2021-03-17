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

### IMPROVEMENTS

### BUG FIXES

- [rpc/jsonrpc/server] \#6191 Correctly unmarshal `RPCRequest` when data is `null` (@melekes)
