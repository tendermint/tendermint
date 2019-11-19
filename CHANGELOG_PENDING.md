## v0.32.8

\*\*

Special thanks to external contributors on this release:
@erikgrinaker, @PSalant726

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

- Go API

  - [libs/pubsub][\#4070](https://github.com/tendermint/tendermint/pull/4070) `Query#(Matches|Conditions)` returns an error.
  - [rpc/client] \#3471 `Validators` now requires two more args: `page` and `perPage`
  - [libs/common] \#3262 Make error the last parameter of `Task` (@PSalant726)
  - [libs/common] \#3862 Remove `errors.go` from `libs/common`

- Blockchain Protocol

  - [abci] \#2521 Remove `TotalTxs` and `NumTxs` from `Header`
  - [types] [\#4151](https://github.com/tendermint/tendermint/pull/4151) Enforce ordering of votes in DuplicateVoteEvidence to be lexicographically sorted on BlockID

- P2P Protocol
  - [p2p][\3668](https://github.com/tendermint/tendermint/pull/3668) Make `SecretConnection` non-malleable

### FEATURES:

### IMPROVEMENTS:

- [mempool][\#4083](https://github.com/tendermint/tendermint/pull/4083) Added TxInfo parameter to CheckTx(), and removed CheckTxWithInfo() (@erikgrinaker)
- [mempool][\#4057](https://github.com/tendermint/tendermint/issues/4057) Include peer ID when logging rejected txns (@erikgrinaker)
- [tools][\#4023](https://github.com/tendermint/tendermint/issues/4023) Improved `tm-monitor` formatting of start time and avg tx throughput (@erikgrinaker)
- [libs/pubsub][\#4070](https://github.com/tendermint/tendermint/pull/4070) No longer panic in `Query#(Matches|Conditions)` preferring to return an error instead.
- [libs/pubsub][\#4070](https://github.com/tendermint/tendermint/pull/4070) Strip out non-numeric characters when attempting to match numeric values.
- [p2p][\#3991](https://github.com/tendermint/tendermint/issues/3991) Log "has been established or dialed" as debug log instead of Error for connected peers (@whunmr)
- [rpc][\#4077](https://github.com/tendermint/tendermint/pull/4077) Added support for `EXISTS` clause to the Websocket query interface.
- [privval] Add `SignerDialerEndpointRetryWaitInterval` option (@cosmostuba)
- [crypto] Add `RegisterKeyType` to amino to allow external key types registration (@austinabell)
- [rpc] \#3188 Added `block_size` to `BlockMeta` this is reflected in `/blockchain`
- [types] \#2521 Add `NumTxs` to `BlockMeta` and `EventDataNewBlockHeader`
- [docs][\#4111](https://github.com/tendermint/tendermint/issues/4111) Replaced dead whitepaper link in README.md

### BUG FIXES:

- [tools][\#4023](https://github.com/tendermint/tendermint/issues/4023) Refresh `tm-monitor` health when validator count is updated (@erikgrinaker)
- [state][\#4104](https://github.com/tendermint/tendermint/pull/4104) txindex/kv: Fsync data to disk immediately after receiving it (@guagualvcha)
- [state][\#4095](https://github.com/tendermint/tendermint/pull/4095) txindex/kv: Return an error if there's one when the user searches for a tx (hash=X) (@hsyis)
- [rpc/lib][\#4051](https://github.com/tendermint/tendermint/pull/4131) Fix RPC client, which was previously resolving https protocol to http (@yenkhoon)
- [rpc][\#4141](https://github.com/tendermint/tendermint/pull/4141) JSONRPCClient: validate that Response.ID matches Request.ID
- [rpc][\#4141](https://github.com/tendermint/tendermint/pull/4141) WSClient: check for unsolicited responses
