/*
# Introduction

Tendermint supports the following RPC protocols:

* URI over HTTP
* JSONRPC over HTTP
* JSONRPC over websockets

Tendermint RPC is built using our own RPC library which contains its own set of documentation and tests.
See it here: https://github.com/tendermint/tendermint/tree/master/rpc/lib

## Configuration

Set the `laddr` config parameter under `[rpc]` table in the `$TMHOME/config/config.toml` file or the `--rpc.laddr` command-line flag to the desired protocol://host:port setting.  Default: `tcp://0.0.0.0:26657`.

## Arguments

Arguments which expect strings or byte arrays may be passed as quoted strings, like `"abc"` or as `0x`-prefixed strings, like `0x616263`.

## URI/HTTP

```bash
curl 'localhost:26657/broadcast_tx_sync?tx="abc"'
```

> Response:

```json
{
	"error": "",
	"result": {
		"hash": "2B8EC32BA2579B3B8606E42C06DE2F7AFA2556EF",
		"log": "",
		"data": "",
		"code": 0
	},
	"id": "",
	"jsonrpc": "2.0"
}
```

## JSONRPC/HTTP

JSONRPC requests can be POST'd to the root RPC endpoint via HTTP (e.g. `http://localhost:26657/`).

```json
{
	"method": "broadcast_tx_sync",
	"jsonrpc": "2.0",
	"params": [ "abc" ],
	"id": "dontcare"
}
```

## JSONRPC/websockets

JSONRPC requests can be made via websocket. The websocket endpoint is at `/websocket`, e.g. `localhost:26657/websocket`.  Asynchronous RPC functions like event `subscribe` and `unsubscribe` are only available via websockets.


## More Examples

See the various bash tests using curl in `test/`, and examples using the `Go` API in `rpc/client/`.

## Get the list

An HTTP Get request to the root RPC endpoint shows a list of available endpoints.

```bash
curl 'localhost:26657'
```

> Response:

```plain
Available endpoints:
/abci_info
/dump_consensus_state
/genesis
/net_info
/num_unconfirmed_txs
/status
/health
/unconfirmed_txs
/unsafe_flush_mempool
/unsafe_stop_cpu_profiler
/validators

Endpoints that require arguments:
/abci_query?path=_&data=_&prove=_
/block?height=_
/blockchain?minHeight=_&maxHeight=_
/broadcast_tx_async?tx=_
/broadcast_tx_commit?tx=_
/broadcast_tx_sync?tx=_
/commit?height=_
/dial_seeds?seeds=_
/dial_persistent_peers?persistent_peers=_
/subscribe?event=_
/tx?hash=_&prove=_
/unsafe_start_cpu_profiler?filename=_
/unsafe_write_heap_profile?filename=_
/unsubscribe?event=_
```

# Endpoints
*/
package core
