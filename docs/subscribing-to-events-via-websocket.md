# Subscribing to events via Websocket

Tendermint emits different events, to which you can subscribe via
[Websocket](https://en.wikipedia.org/wiki/WebSocket). This can be useful
for third-party applications (for analysys) or inspecting state.

[List of events](https://godoc.org/github.com/tendermint/tendermint/types#pkg-constants)

You can subscribe to any of the events above by calling `subscribe` RPC
method via Websocket.

    {
        "jsonrpc": "2.0",
        "method": "subscribe",
        "id": "0",
        "params": {
            "query": "tm.event='NewBlock'"
        }
    }

Check out [API docs](https://tendermint.github.io/slate/#subscribe) for
more information on query syntax and other options.

You can also use tags, given you had included them into DeliverTx
response, to query transaction results. See [Indexing
transactions](./indexing-transactions.md) for details.
