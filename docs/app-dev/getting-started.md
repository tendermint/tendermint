# Getting Started

## First Tendermint App

As a general purpose blockchain engine, Tendermint is agnostic to the
application you want to run. So, to run a complete blockchain that does
something useful, you must start two programs: one is Tendermint Core,
the other is your application, which can be written in any programming
language. Recall from [the intro to
ABCI](../introduction/what-is-tendermint.md#abci-overview) that Tendermint Core handles all the p2p and consensus stuff, and just forwards transactions to the
application when they need to be validated, or when they're ready to be
committed to a block.

In this guide, we show you some examples of how to run an application
using Tendermint.

### Install

The first apps we will work with are written in Go. To install them, you
need to [install Go](https://golang.org/doc/install) and put
`$GOPATH/bin` in your `$PATH`; see
[here](https://github.com/tendermint/tendermint/wiki/Setting-GOPATH) for
more info.

Then run

```
go get github.com/tendermint/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint
make tools
make install_abci
```

Now you should have the `abci-cli` installed; you'll see a couple of
commands (`counter` and `kvstore`) that are example applications written
in Go. See below for an application written in JavaScript.

Now, let's run some apps!

## KVStore - A First Example

The kvstore app is a [Merkle
tree](https://en.wikipedia.org/wiki/Merkle_tree) that just stores all
transactions. If the transaction contains an `=`, e.g. `key=value`, then
the `value` is stored under the `key` in the Merkle tree. Otherwise, the
full transaction bytes are stored as the key and the value.

Let's start a kvstore application.

```
abci-cli kvstore
```

In another terminal, we can start Tendermint. If you have never run
Tendermint before, use:

```
tendermint init
tendermint node
```

If you have used Tendermint, you may want to reset the data for a new
blockchain by running `tendermint unsafe_reset_all`. Then you can run
`tendermint node` to start Tendermint, and connect to the app. For more
details, see [the guide on using Tendermint](../tendermint-core/using-tendermint.md).

You should see Tendermint making blocks! We can get the status of our
Tendermint node as follows:

```
curl -s localhost:26657/status
```

The `-s` just silences `curl`. For nicer output, pipe the result into a
tool like [jq](https://stedolan.github.io/jq/) or `json_pp`.

Now let's send some transactions to the kvstore.

```
curl -s 'localhost:26657/broadcast_tx_commit?tx="abcd"'
```

Note the single quote (`'`) around the url, which ensures that the
double quotes (`"`) are not escaped by bash. This command sent a
transaction with bytes `abcd`, so `abcd` will be stored as both the key
and the value in the Merkle tree. The response should look something
like:

```
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "check_tx": {},
    "deliver_tx": {
      "tags": [
        {
          "key": "YXBwLmNyZWF0b3I=",
          "value": "amFl"
        },
        {
          "key": "YXBwLmtleQ==",
          "value": "YWJjZA=="
        }
      ]
    },
    "hash": "9DF66553F98DE3C26E3C3317A3E4CED54F714E39",
    "height": 14
  }
}
```

We can confirm that our transaction worked and the value got stored by
querying the app:

```
curl -s 'localhost:26657/abci_query?data="abcd"'
```

The result should look like:

```
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "response": {
      "log": "exists",
      "index": "-1",
      "key": "YWJjZA==",
      "value": "YWJjZA=="
    }
  }
}
```

Note the `value` in the result (`YWJjZA==`); this is the base64-encoding
of the ASCII of `abcd`. You can verify this in a python 2 shell by
running `"YWJjZA==".decode('base64')` or in python 3 shell by running
`import codecs; codecs.decode(b"YWJjZA==", 'base64').decode('ascii')`.
Stay tuned for a future release that [makes this output more
human-readable](https://github.com/tendermint/tendermint/issues/1794).

Now let's try setting a different key and value:

```
curl -s 'localhost:26657/broadcast_tx_commit?tx="name=satoshi"'
```

Now if we query for `name`, we should get `satoshi`, or `c2F0b3NoaQ==`
in base64:

```
curl -s 'localhost:26657/abci_query?data="name"'
```

Try some other transactions and queries to make sure everything is
working!

## Counter - Another Example

Now that we've got the hang of it, let's try another application, the
`counter` app.

The counter app doesn't use a Merkle tree, it just counts how many times
we've sent a transaction, or committed the state.

This application has two modes: `serial=off` and `serial=on`.

When `serial=on`, transactions must be a big-endian encoded incrementing
integer, starting at 0.

If `serial=off`, there are no restrictions on transactions.

In a live blockchain, transactions collect in memory before they are
committed into blocks. To avoid wasting resources on invalid
transactions, ABCI provides the `CheckTx` message, which application
developers can use to accept or reject transactions, before they are
stored in memory or gossipped to other peers.

In this instance of the counter app, with `serial=on`, `CheckTx` only
allows transactions whose integer is greater than the last committed
one.

Let's kill the previous instance of `tendermint` and the `kvstore`
application, and start the counter app. We can enable `serial=on` with a
flag:

```
abci-cli counter --serial
```

In another window, reset then start Tendermint:

```
tendermint unsafe_reset_all
tendermint node
```

Once again, you can see the blocks streaming by. Let's send some
transactions. Since we have set `serial=on`, the first transaction must
be the number `0`:

```
curl localhost:26657/broadcast_tx_commit?tx=0x00
```

Note the empty (hence successful) response. The next transaction must be
the number `1`. If instead, we try to send a `5`, we get an error:

```
> curl localhost:26657/broadcast_tx_commit?tx=0x05
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "check_tx": {},
    "deliver_tx": {
      "code": 2,
      "log": "Invalid nonce. Expected 1, got 5"
    },
    "hash": "33B93DFF98749B0D6996A70F64071347060DC19C",
    "height": 34
  }
}
```

But if we send a `1`, it works again:

```
> curl localhost:26657/broadcast_tx_commit?tx=0x01
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "check_tx": {},
    "deliver_tx": {},
    "hash": "F17854A977F6FA7EEA1BD758E296710B86F72F3D",
    "height": 60
  }
}
```

For more details on the `broadcast_tx` API, see [the guide on using
Tendermint](../tendermint-core/using-tendermint.md).

## CounterJS - Example in Another Language

We also want to run applications in another language - in this case,
we'll run a Javascript version of the `counter`. To run it, you'll need
to [install node](https://nodejs.org/en/download/).

You'll also need to fetch the relevant repository, from
[here](https://github.com/tendermint/js-abci), then install it:

```
git clone https://github.com/tendermint/js-abci.git
cd js-abci
npm install abci
```

Kill the previous `counter` and `tendermint` processes. Now run the app:

```
node example/counter.js
```

In another window, reset and start `tendermint`:

```
tendermint unsafe_reset_all
tendermint node
```

Once again, you should see blocks streaming by - but now, our
application is written in Javascript! Try sending some transactions, and
like before - the results should be the same:

```
# ok
curl localhost:26657/broadcast_tx_commit?tx=0x00
# invalid nonce
curl localhost:26657/broadcast_tx_commit?tx=0x05
# ok
curl localhost:26657/broadcast_tx_commit?tx=0x01
```

Neat, eh?
