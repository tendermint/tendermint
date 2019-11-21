## v0.32.9

\*\*

Special thanks to external contributors on this release:
@erikgrinaker, @PSalant726, @gchaincl, @gregzaitsev

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

  - [rpc] \#3471 Paginate `/validators` response (default: 30 vals per page)
  - [rpc] \#3188 Remove `BlockMeta` in `ResultBlock` in favor of `BlockId` for `/block`
  - [rpc] `/block_results` response format updated (see RPC docs for details)
    ```
    {
      "jsonrpc": "2.0",
      "id": "",
      "result": {
        "height": "2109",
        "txs_results": null,
        "begin_block_events": null,
        "end_block_events": null,
        "validator_updates": null,
        "consensus_param_updates": null
      }
    }
    ```
  - [rpc][\#4141](https://github.com/tendermint/tendermint/pull/4141) Remove `#event` suffix from the ID in event responses.
    `{"jsonrpc": "2.0", "id": 0, "result": ...}`
  - [rpc][\#4141](https://github.com/tendermint/tendermint/pull/4141) Switch to integer IDs instead of `json-client-XYZ`
    ```
    id=0 method=/subscribe
    id=0 result=...
    id=1 method=/abci_query
    id=1 result=...
    ```
    - ID is unique for each request;
    - Request.ID is now optional. Notification is a Request without an ID. Previously ID="" or ID=0 were considered as notifications.

- Apps

- [tm-bench] Removed tm-bench in favor of [tm-load-test](https://github.com/interchainio/tm-load-test)

- Go API

  - [rpc/client] \#3471 `Validators` now requires two more args: `page` and `perPage`
  - [libs/common] \#3262 Make error the last parameter of `Task` (@PSalant726)
  - [libs/common] \#3862 Remove `errors.go` from `libs/common`

- Blockchain Protocol

  - [abci] \#2521 Remove `TotalTxs` and `NumTxs` from `Header`
  - [types][\#4151](https://github.com/tendermint/tendermint/pull/4151) Enforce ordering of votes in DuplicateVoteEvidence to be lexicographically sorted on BlockID

- P2P Protocol
  - [p2p][\3668](https://github.com/tendermint/tendermint/pull/3668) Make `SecretConnection` non-malleable

- [proto] [\#3986](https://github.com/tendermint/tendermint/pull/3986) Prefix protobuf types to avoid name conflicts.
  - ABCI becomes `tendermint.abci.types` with the new API endpoint `/tendermint.abci.types.ABCIApplication/`
  - core_grpc becomes `tendermint.rpc.grpc` with the new API endpoint `/tendermint.rpc.grpc.BroadcastAPI/`
  - merkle becomes `tendermint.crypto.merkle`
  - libs.common becomes `tendermint.libs.common`
  - proto3 becomes `tendermint.types.proto3`

### FEATURES:

### IMPROVEMENTS:

- [rpc] \#3188 Added `block_size` to `BlockMeta` this is reflected in `/blockchain`
- [types] \#2521 Add `NumTxs` to `BlockMeta` and `EventDataNewBlockHeader`
- [docs][\#4111](https://github.com/tendermint/tendermint/issues/4111) Replaced dead whitepaper link in README.md

### BUG FIXES:

- [rpc/lib][\#4051](https://github.com/tendermint/tendermint/pull/4131) Fix RPC client, which was previously resolving https protocol to http (@yenkhoon)
- [rpc][\#4141](https://github.com/tendermint/tendermint/pull/4141) JSONRPCClient: validate that Response.ID matches Request.ID
- [rpc][\#4141](https://github.com/tendermint/tendermint/pull/4141) WSClient: check for unsolicited responses
- [types][\4164](https://github.com/tendermint/tendermint/pull/4164) Prevent temporary power overflows on validator updates
- [cs] \#4069 Don't panic when block meta is not found in store (@gregzaitsev)
