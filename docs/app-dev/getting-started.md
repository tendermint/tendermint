---
order: 1
---

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
need to [install Go](https://golang.org/doc/install), put
`$GOPATH/bin` in your `$PATH` and enable go modules with these instructions:

```bash
echo export GOPATH=\"\$HOME/go\" >> ~/.bash_profile
echo export PATH=\"\$PATH:\$GOPATH/bin\" >> ~/.bash_profile
```

Then run

```sh
go get github.com/tendermint/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint
make install_abci
```

Now you should have the `abci-cli` installed; you'll notice the `kvstore`
command, an example application written
in Go. See below for an application written in JavaScript.

Now, let's run some apps!

## KVStore - A First Example

The kvstore app is a [Merkle
tree](https://en.wikipedia.org/wiki/Merkle_tree) that just stores all
transactions. If the transaction contains an `=`, e.g. `key=value`, then
the `value` is stored under the `key` in the Merkle tree. Otherwise, the
full transaction bytes are stored as the key and the value.

Let's start a kvstore application.

```sh
abci-cli kvstore
```

In another terminal, we can start Tendermint. You should already have the
Tendermint binary installed. If not, follow the steps from
[here](../introduction/install.md). If you have never run Tendermint
before, use:

```sh
tendermint init validator
tendermint start
```

If you have used Tendermint, you may want to reset the data for a new
blockchain by running `tendermint unsafe-reset-all`. Then you can run
`tendermint start` to start Tendermint, and connect to the app. For more
details, see [the guide on using Tendermint](../tendermint-core/using-tendermint.md).

You should see Tendermint making blocks! We can get the status of our
Tendermint node as follows:

```sh
curl -s localhost:26657/status
```

The `-s` just silences `curl`. For nicer output, pipe the result into a
tool like [jq](https://stedolan.github.io/jq/) or `json_pp`.

Now let's send some transactions to the kvstore.

```sh
curl -s 'localhost:26657/broadcast_tx_commit?tx="abcd"'
```

Note the single quote (`'`) around the url, which ensures that the
double quotes (`"`) are not escaped by bash. This command sent a
transaction with bytes `abcd`, so `abcd` will be stored as both the key
and the value in the Merkle tree. The response should look something
like:

```json
{
  "check_tx": { ... },
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
```

We can confirm that our transaction worked and the value got stored by
querying the app:

```sh
curl -s 'localhost:26657/abci_query?data="abcd"'
```

The result should look like:

```json
{
  "response": {
    "log": "exists",
    "index": "-1",
    "key": "YWJjZA==",
    "value": "YWJjZA=="
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

```sh
curl -s 'localhost:26657/broadcast_tx_commit?tx="name=satoshi"'
```

Now if we query for `name`, we should get `satoshi`, or `c2F0b3NoaQ==`
in base64:

```sh
curl -s 'localhost:26657/abci_query?data="name"'
```

Try some other transactions and queries to make sure everything is
working!


## CounterJS - Example in Another Language

We also want to run applications in another language - in this case,
we'll run a Javascript version of the `counter`. To run it, you'll need
to [install node](https://nodejs.org/en/download/).

You'll also need to fetch the relevant repository, from
[here](https://github.com/tendermint/js-abci), then install it:

```sh
git clone https://github.com/tendermint/js-abci.git
cd js-abci
npm install abci
```

Kill the previous `counter` and `tendermint` processes. Now run the app:

```sh
node example/counter.js
```

In another window, reset and start `tendermint`:

```sh
tendermint reset unsafe-all
tendermint start
```

Once again, you should see blocks streaming by - but now, our
application is written in Javascript! Try sending some transactions, and
like before - the results should be the same:

```sh
# ok
curl localhost:26657/broadcast_tx_commit?tx=0x00
# invalid nonce
curl localhost:26657/broadcast_tx_commit?tx=0x05
# ok
curl localhost:26657/broadcast_tx_commit?tx=0x01
```

Neat, eh?
