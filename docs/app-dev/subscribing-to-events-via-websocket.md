# Subscribing to events via Websocket

Tendermint emits different events, to which you can subscribe via
[Websocket](https://en.wikipedia.org/wiki/WebSocket). This can be useful
for third-party applications (for analysis) or inspecting state.

[List of events](https://godoc.org/github.com/tendermint/tendermint/types#pkg-constants)

You can subscribe to any of the events above by calling `subscribe` RPC
method via Websocket.

```
{
    "jsonrpc": "2.0",
    "method": "subscribe",
    "id": "0",
    "params": {
        "query": "tm.event='NewBlock'"
    }
}
```

Check out [API docs](https://tendermint.com/rpc/) for
more information on query syntax and other options.

You can also use tags, given you had included them into DeliverTx
response, to query transaction results. See [Indexing
transactions](./indexing-transactions.md) for details.

### ValidatorSetUpdates

When validator set changes, ValidatorSetUpdates event is published. The
event carries a list of pubkey/power pairs. The list is the same
Tendermint receives from ABCI application (see [EndBlock
section](../spec/abci/abci.md#endblock) in
the ABCI spec).

Response:

```
{
    "jsonrpc": "2.0",
    "id": "0#event",
    "result": {
        "query": "tm.event='ValidatorSetUpdates'",
        "data": {
            "type": "tendermint/event/ValidatorSetUpdates",
            "value": {
              "validator_updates": [
                {
                  "address": "09EAD022FD25DE3A02E64B0FE9610B1417183EE4",
                  "pub_key": {
                    "type": "tendermint/PubKeyEd25519",
                    "value": "ww0z4WaZ0Xg+YI10w43wTWbBmM3dpVza4mmSQYsd0ck="
                  },
                  "voting_power": "10",
                  "proposer_priority": "0"
                }
              ]
            }
        }
    }
}
```
