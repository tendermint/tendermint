# Unreleased Changes

## v0.34.9

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API
<<<<<<< HEAD
=======
  - [abci/client, proxy] \#5673 `Async` funcs return an error, `Sync` and `Async` funcs accept `context.Context` (@melekes)
  - [p2p] Removed unused function `MakePoWTarget`. (@erikgrinaker)
  - [libs/bits] \#5720 Validate `BitArray` in `FromProto`, which now returns an error (@melekes)
  - [proto/p2p] Renamed `DefaultNodeInfo` and `DefaultNodeInfoOther` to `NodeInfo` and `NodeInfoOther` (@erikgrinaker)
  - [proto/p2p] Rename `NodeInfo.default_node_id` to `node_id` (@erikgrinaker)
  - [libs/os] Kill() and {Must,}{Read,Write}File() functions have been removed. (@alessio)
  - [store] \#5848 Remove block store state in favor of using the db iterators directly (@cmwaters)
  - [state] \#5864 Use an iterator when pruning state (@cmwaters)
  - [types] \#6023 Remove `tm2pb.Header`, `tm2pb.BlockID`, `tm2pb.PartSetHeader` and `tm2pb.NewValidatorUpdate`.
    - Each of the above types has a `ToProto` and `FromProto` method or function which replaced this logic.
  - [light] \#6054 Move `MaxRetryAttempt` option from client to provider.
    - `NewWithOptions` now sets the max retry attempts and timeouts (@cmwaters)
  - [all] \#6077 Change spelling from British English to American (@cmwaters)
    - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub
    - Rename "behaviour" pkg to "behavior" and internalized it in blockchain v2
  - [rpc/client/http] \#6176 Remove `endpoint` arg from `New`, `NewWithTimeout` and `NewWithClient` (@melekes)
  - [rpc/client/http] \#6176 Unexpose `WSEvents` (@melekes)
  - [rpc/jsonrpc/client/ws_client] \#6176 `NewWS` no longer accepts options (use `NewWSWithOptions` and `OnReconnect` funcs to configure the client) (@melekes)
  - [rpc/jsonrpc/server] \#6204 Modify `WriteRPCResponseHTTP(Error)` to return an error (@melekes)
>>>>>>> 00b952416... rpc/jsonrpc/server: return an error in WriteRPCResponseHTTP(Error) (#6204)

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

### BUG FIXES

- [rpc/jsonrpc/server] \#6191 Correctly unmarshal `RPCRequest` when data is `null` (@melekes)
