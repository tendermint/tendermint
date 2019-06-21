# Indexing Transactions

Tendermint allows you to index transactions and later query or subscribe
to their results.

Let's take a look at the `[tx_index]` config section:

```
##### transactions indexer configuration options #####
[tx_index]

# What indexer to use for transactions
#
# Options:
#   1) "null"
#   2) "kv" (default) - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
indexer = "kv"

# Comma-separated list of tags to index (by default the only tag is "tx.hash")
#
# You can also index transactions by height by adding "tx.height" tag here.
#
# It's recommended to index only a subset of tags due to possible memory
# bloat. This is, of course, depends on the indexer's DB and the volume of
# transactions.
index_tags = ""

# When set to true, tells indexer to index all tags (predefined tags:
# "tx.hash", "tx.height" and all tags from DeliverTx responses).
#
# Note this may be not desirable (see the comment above). IndexTags has a
# precedence over IndexAllTags (i.e. when given both, IndexTags will be
# indexed).
index_all_tags = false
```

By default, Tendermint will index all transactions by their respective
hashes using an embedded simple indexer. Note, we are planning to add
more options in the future (e.g., Postgresql indexer).

## Adding tags

In your application's `DeliverTx` method, add the `Tags` field with the
pairs of UTF-8 encoded strings (e.g. "account.owner": "Bob", "balance":
"100.0", "date": "2018-01-02").

Example:

```
func (app *KVStoreApplication) DeliverTx(req types.RequestDeliverTx) types.Result {
    ...
    tags := []cmn.KVPair{
      {[]byte("account.name"), []byte("igor")},
      {[]byte("account.address"), []byte("0xdeadbeef")},
      {[]byte("tx.amount"), []byte("7")},
    }
    return types.ResponseDeliverTx{Code: code.CodeTypeOK, Tags: tags}
}
```

If you want Tendermint to only index transactions by "account.name" tag,
in the config set `tx_index.index_tags="account.name"`. If you to index
all tags, set `index_all_tags=true`

Note, there are a few predefined tags:

- `tx.hash` (transaction's hash)
- `tx.height` (height of the block transaction was committed in)

Tendermint will throw a warning if you try to use any of the above keys.

## Querying transactions

You can query the transaction results by calling `/tx_search` RPC
endpoint:

```
curl "localhost:26657/tx_search?query=\"account.name='igor'\"&prove=true"
```

Check out [API docs](https://tendermint.com/rpc/#txsearch)
for more information on query syntax and other options.

## Subscribing to transactions

Clients can subscribe to transactions with the given tags via Websocket
by providing a query to `/subscribe` RPC endpoint.

```
{
    "jsonrpc": "2.0",
    "method": "subscribe",
    "id": "0",
    "params": {
        "query": "account.name='igor'"
    }
}
```

Check out [API docs](https://tendermint.com/rpc/#subscribe) for
more information on query syntax and other options.
