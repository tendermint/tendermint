# Mempool Messages

## P2P Messages

There is currently only one message that Mempool broadcasts
and receives over the p2p gossip network (via the reactor):
`TxMessage`

```go
// TxMessage is a MempoolMessage containing a transaction.
type TxMessage struct {
    Tx types.Tx
}
```

TxMessage is go-amino encoded and prepended with `0x1` as a
"type byte". This is followed by a go-amino encoded byte-slice.
Prefix of 40=0x28 byte tx is: `0x010128...` followed by
the actual 40-byte tx. Prefix of 350=0x015e byte tx is:
`0x0102015e...` followed by the actual 350 byte tx.

(Please see the [go-amino repo](https://github.com/tendermint/go-amino#an-interface-example) for more information)

## RPC Messages

Mempool exposes `CheckTx([]byte)` over the RPC interface.

It can be posted via `broadcast_commit`, `broadcast_sync` or
`broadcast_async`. They all parse a message with one argument,
`"tx": "HEX_ENCODED_BINARY"` and differ in only how long they
wait before returning (sync makes sure CheckTx passes, commit
makes sure it was included in a signed block).

Request (`POST http://gaia.zone:26657/`):

```json
{
  "id": "",
  "jsonrpc": "2.0",
  "method": "broadcast_sync",
  "params": {
    "tx": "F012A4BC68..."
  }
}
```

Response:

```json
{
  "error": "",
  "result": {
    "hash": "E39AAB7A537ABAA237831742DCE1117F187C3C52",
    "log": "",
    "data": "",
    "code": 0
  },
  "id": "",
  "jsonrpc": "2.0"
}
```
