---
order: 6
---

# Indexing Transactions

Tendermint allows you to index transactions and later query or subscribe
to their results.

Events can be used to index transactions and blocks according to what happened
during their execution. Note that the set of events returned for a block from
`BeginBlock` and `EndBlock` are merged. In case both methods return the same
type, only the key-value pairs defined in `EndBlock` are used.

Each event contains a type and a list of attributes, which are key-value pairs
denoting something about what happened during the method's execution. For more
details on `Events`, see the [ABCI]https://github.com/tendermint/spec/blob/master/spec/abci/abci.md#events) documentation.

An Event has a composite key associated with it. A `compositeKey` is constructed by its type and key separated by a dot.
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

# Comma-separated list of composite keys to index (by default the only key is "tx.hash")
#
# You can also index transactions by height by adding "tx.height" key here.
#
# It's recommended to index only a subset of keys due to possible memory
# bloat. This is, of course, depends on the indexer's DB and the volume of
# transactions.
index_keys = ""

# When set to true, tells indexer to index all compositeKeys (predefined keys:
# "tx.hash", "tx.height" and all keys from DeliverTx responses).
#
# Note this may be not desirable (see the comment above). Indexkeys has a
# precedence over IndexAllKeys (i.e. when given both, IndexKeys will be
# indexed).
index_all_keys = false
```

By default, Tendermint will index all transactions by their respective
hashes using an embedded simple indexer. Note, we are planning to add
more options in the future (e.g., PostgreSQL indexer).

## Adding Events

In your application's `DeliverTx` method, add the `Events` field with pairs of
UTF-8 encoded strings (e.g. "transfer.sender": "Bob", "transfer.recipient": "Alice",
"transfer.balance": "100").

Example:

```go
func (app *KVStoreApplication) DeliverTx(req types.RequestDeliverTx) types.Result {
    //...
    events := []abci.Event{
        {
            Type: "transfer",
            Attributes: kv.Pairs{
                kv.Pair{Key: []byte("sender"), Value: []byte("Bob")},
                kv.Pair{Key: []byte("recipient"), Value: []byte("Alice")},
                kv.Pair{Key: []byte("balance"), Value: []byte("100")},
            },
        },
    }
    return types.ResponseDeliverTx{Code: code.CodeTypeOK, Events: events}
}
```

If you want Tendermint to only index transactions by "transfer.sender" event type,
in the config set `tx_index.index_tags="transfer.sender"`. If you to index all events,
set `index_all_tags=true`

Note, there are a few predefined event types:

- `tx.hash` (transaction's hash)
- `tx.height` (height of the block transaction was committed in)

Tendermint will throw a warning if you try to use any of the above keys.

## Querying Transactions

You can query the transaction results by calling `/tx_search` RPC endpoint:

```shell
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
