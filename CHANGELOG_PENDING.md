## v0.32.8

\*\*

Special thanks to external contributors on this release:
@erikgrinaker

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

- Apps

- Go API
  - [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) `Query#(Matches|Conditions)` returns an error.
  - [rpc/client] \#3471 `Validators` now requires two more args: `page` and `perPage`

- Blockchain Protocol
  - [abci] \#2521 Remove `TotalTxs` and `NumTxs` from `Header`

- P2P Protocol
  - [p2p] [\3668](https://github.com/tendermint/tendermint/pull/3668) Make `SecretConnection` non-malleable

### FEATURES:

### IMPROVEMENTS:

- [mempool] [\#4083](https://github.com/tendermint/tendermint/pull/4083) Added TxInfo parameter to CheckTx(), and removed CheckTxWithInfo() (@erikgrinaker)
- [mempool] [\#4057](https://github.com/tendermint/tendermint/issues/4057) Include peer ID when logging rejected txns (@erikgrinaker)
- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Improved `tm-monitor` formatting of start time and avg tx throughput (@erikgrinaker)
- [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) No longer panic in `Query#(Matches|Conditions)` preferring to return an error instead.
- [libs/pubsub] [\#4070](https://github.com/tendermint/tendermint/pull/4070) Strip out non-numeric characters when attempting to match numeric values.
- [p2p] [\#3991](https://github.com/tendermint/tendermint/issues/3991) Log "has been established or dialed" as debug log instead of Error for connected peers (@whunmr)
- [rpc] [\#4077](https://github.com/tendermint/tendermint/pull/4077) Added support for `EXISTS` clause to the Websocket query interface.
- [privval] Add `SignerDialerEndpointRetryWaitInterval` option (@cosmostuba)
- [crypto] Add `RegisterKeyType` to amino to allow external key types registration (@austinabell)
- [rpc] \#3188 Added `block_size` to `BlockMeta` this is reflected in `/blockchain`
- [types] \#2521 Add `NumTxs` to `BlockMeta` and `EventDataNewBlockHeader`

### BUG FIXES:

- [tools] [\#4023](https://github.com/tendermint/tendermint/issues/4023) Refresh `tm-monitor` health when validator count is updated (@erikgrinaker)
- [state] [\#4104](https://github.com/tendermint/tendermint/pull/4104) txindex/kv: Fsync data to disk immediately after receiving it (@guagualvcha)
- [state] [\#4095](https://github.com/tendermint/tendermint/pull/4095) txindex/kv: Return an error if there's one when the user searches for a tx (hash=X) (@hsyis)
- [rpc/lib] [\#4051](https://github.com/tendermint/tendermint/pull/4131) Fix RPC client, which was previously resolving https protocol to http (@yenkhoon)
