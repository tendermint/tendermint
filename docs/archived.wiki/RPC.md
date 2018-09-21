NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

Tendermint supports the following RPC protocols:
* URI over HTTP
* JSONRPC over HTTP
* JSONRPC over websockets

### Configuration

Set the `rpc_laddr` config parameter in the `$TMHOME/config/config.toml` file or the `--rpc-laddr` command-line flag to the desired listener:port setting.  Default: `0.0.0.0:46657`.

### URI/HTTP

Example request:
```bash
TXBYTES="<HEXBYTES>"
curl "http://localhost:46657/broadcast_tx_sync?tx=%22$TXBYTES%22" | jq
```

Response:
```json
{"jsonrpc":"2.0","id":"","result":[96,{"code":0,"data":"","log":""}],"error":""}

```
The first entry in the result-array (96) is the method this response correlates with. 96 refers to "ResultTypeBroadcastTx", see [responses.go](https://github.com/tendermint/tendermint/blob/master/rpc/core/types/responses.go) for a complete overview. 


### JSONRPC/HTTP

JSONRPC requests can be POST'd to the root RPC endpoint via HTTP (e.g. `http://localhost:46657/`).

Example request:
```json
{
  "method": "broadcast_tx_sync",
  "jsonrpc": "2.0",
  "params": [ "<HEXBYTES>" ],
  "id": "dontcare"
}
```

### JSONRPC/websockets

JSONRPC requests can be made via websocket.  The websocket endpoint is at `/websocket`, e.g. `http://localhost:46657/websocket`.  Asynchronous RPC functions like event `subscribe` and `unsubscribe` are only available via websockets.

NOTE: see [this issue](https://github.com/tendermint/tendermint/issues/536#issuecomment-312788905) for additional information on transaction formats.

### Endpoints

An HTTP Get request to the root RPC endpoint (e.g. `http://localhost:46657`) shows a list of available endpoints. 

```
Available endpoints:
http://localhost:46657/dump_consensus_state
http://localhost:46657/genesis
http://localhost:46657/net_info
http://localhost:46657/num_unconfirmed_txs
http://localhost:46657/status
http://localhost:46657/unconfirmed_txs
http://localhost:46657/unsafe_stop_cpu_profiler
http://localhost:46657/validators

Endpoints that require arguments:
http://localhost:46657/block?height=_
http://localhost:46657/blockchain?minHeight=_&maxHeight=_
http://localhost:46657/broadcast_tx_async?tx=_
http://localhost:46657/broadcast_tx_sync?tx=_
http://localhost:46657/dial_seeds?seeds=_
http://localhost:46657/subscribe?event=_
http://localhost:46657/unsafe_set_config?type=_&key=_&value=_
http://localhost:46657/unsafe_start_cpu_profiler?filename=_
http://localhost:46657/unsafe_write_heap_profile?filename=_
http://localhost:46657/unsubscribe?event=_
```

### More Examples

See the various bash tests using curl in `test/`, and examples using the `Go` API in `rpc/test/`. Tendermint uses the [go-rpc](https://github.com/tendermint/go-rpc) library, with docs at https://godoc.org/github.com/tendermint/go-rpc/client.