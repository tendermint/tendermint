---
order: 6
---

# Indexing Transactions

Tendermint allows you to index transactions and later query or subscribe to their results.

Events can be used to index transactions and blocks according to what happened
during their execution. Note that the set of events returned for a block from
`BeginBlock` and `EndBlock` are merged. In case both methods return the same
type, only the key-value pairs defined in `EndBlock` are used.

Each event contains a type and a list of attributes, which are key-value pairs
denoting something about what happened during the method's execution. For more
details on `Events`, see the
[ABCI](https://github.com/tendermint/spec/blob/master/spec/abci/abci.md#events)
documentation.

An Event has a composite key associated with it. A `compositeKey` is
constructed by its type and key separated by a dot.

For example:

```json
"jack": [
  "account.number": 100
]
```

would be equal to the composite key of `jack.account.number`.

Let's take a look at the `[tx_index]` config section:

```toml
##### transactions indexer configuration options #####
[tx_index]

# What indexer to use for transactions
#
# Options:
#   1) "null"
#   2) "kv" (default) - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
indexer = "kv"
```

By default, Tendermint will index all transactions by their respective
hashes and height using an embedded simple indexer.

You can turn off indexing completely by setting `tx_index` to `null`.

## Adding Events

Applications are free to define which events to index. Tendermint does not
expose functionality to define which events to index and which to ignore. In
your application's `DeliverTx` method, add the `Events` field with pairs of
UTF-8 encoded strings (e.g. "transfer.sender": "Bob", "transfer.recipient":
"Alice", "transfer.balance": "100").

Example:

```go
func (app *KVStoreApplication) DeliverTx(req types.RequestDeliverTx) types.Result {
    //...
    events := []abci.Event{
        {
            Type: "transfer",
            Attributes: []abci.EventAttribute{
                {Key: []byte("sender"), Value: []byte("Bob"), Index: true},
                {Key: []byte("recipient"), Value: []byte("Alice"), Index: true},
                {Key: []byte("balance"), Value: []byte("100"), Index: true},
                {Key: []byte("note"), Value: []byte("nothing"), Index: true},
            },
        },
    }
    return types.ResponseDeliverTx{Code: code.CodeTypeOK, Events: events}
}
```

The transaction will be indexed (if the indexer is not `null`) with a certain attribute if the attribute's `Index` field is set to `true`. 
In the above example, all attributes will be indexed.

## Querying Transactions

You can query the transaction results by calling `/tx_search` RPC endpoint:

```bash
curl "localhost:26657/tx_search?query=\"account.name='igor'\"&prove=true"
```

Check out [API docs](https://docs.tendermint.com/master/rpc/#/Info/tx_search) for more information
on query syntax and other options.

## Subscribing to Transactions

Clients can subscribe to transactions with the given tags via WebSocket by providing
a query to `/subscribe` RPC endpoint.

```json
{
  "jsonrpc": "2.0",
  "method": "subscribe",
  "id": "0",
  "params": {
    "query": "account.name='igor'"
  }
}
```

Check out [API docs](https://docs.tendermint.com/master/rpc/#subscribe) for more information
on query syntax and other options.
