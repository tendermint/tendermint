# Using ABCI-CLI

To facilitate testing and debugging of ABCI servers and simple apps, we
built a CLI, the `abci-cli`, for sending ABCI messages from the command
line.

## Install

Make sure you [have Go installed](https://golang.org/doc/install).

Next, install the `abci-cli` tool and example applications:

```
mkdir -p $GOPATH/src/github.com/tendermint
cd $GOPATH/src/github.com/tendermint
git clone https://github.com/tendermint/tendermint.git
cd tendermint
make get_tools
make get_vendor_deps
make install_abci
```

Now run `abci-cli` to see the list of commands:

```
Usage:
  abci-cli [command]

Available Commands:
  batch       Run a batch of abci commands against an application
  check_tx    Validate a tx
  commit      Commit the application state and return the Merkle root hash
  console     Start an interactive abci console for multiple commands
  counter     ABCI demo example
  deliver_tx  Deliver a new tx to the application
  kvstore     ABCI demo example
  echo        Have the application echo a message
  help        Help about any command
  info        Get some info about the application
  query       Query the application state
  set_option  Set an options on the application

Flags:
      --abci string      socket or grpc (default "socket")
      --address string   address of application socket (default "tcp://127.0.0.1:26658")
  -h, --help             help for abci-cli
  -v, --verbose          print the command and results as if it were a console session

Use "abci-cli [command] --help" for more information about a command.
```

## KVStore - First Example

The `abci-cli` tool lets us send ABCI messages to our application, to
help build and debug them.

The most important messages are `deliver_tx`, `check_tx`, and `commit`,
but there are others for convenience, configuration, and information
purposes.

We'll start a kvstore application, which was installed at the same time
as `abci-cli` above. The kvstore just stores transactions in a merkle
tree.

Its code can be found
[here](https://github.com/tendermint/tendermint/blob/develop/abci/cmd/abci-cli/abci-cli.go)
and looks like:

```
func cmdKVStore(cmd *cobra.Command, args []string) error {
    logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

    // Create the application - in memory or persisted to disk
    var app types.Application
    if flagPersist == "" {
        app = kvstore.NewKVStoreApplication()
    } else {
        app = kvstore.NewPersistentKVStoreApplication(flagPersist)
        app.(*kvstore.PersistentKVStoreApplication).SetLogger(logger.With("module", "kvstore"))
    }

    // Start the listener
    srv, err := server.NewServer(flagAddrD, flagAbci, app)
    if err != nil {
        return err
    }
    srv.SetLogger(logger.With("module", "abci-server"))
    if err := srv.Start(); err != nil {
        return err
    }

    // Wait forever
    cmn.TrapSignal(func() {
        // Cleanup
        srv.Stop()
    })
    return nil
}
```

Start by running:

```
abci-cli kvstore
```

And in another terminal, run

```
abci-cli echo hello
abci-cli info
```

You'll see something like:

```
-> data: hello
-> data.hex: 68656C6C6F
```

and:

```
-> data: {"size":0}
-> data.hex: 7B2273697A65223A307D
```

An ABCI application must provide two things:

- a socket server
- a handler for ABCI messages

When we run the `abci-cli` tool we open a new connection to the
application's socket server, send the given ABCI message, and wait for a
response.

The server may be generic for a particular language, and we provide a
[reference implementation in
Golang](https://github.com/tendermint/tendermint/tree/develop/abci/server). See the
[list of other ABCI implementations](./ecosystem.md) for servers in
other languages.

The handler is specific to the application, and may be arbitrary, so
long as it is deterministic and conforms to the ABCI interface
specification.

So when we run `abci-cli info`, we open a new connection to the ABCI
server, which calls the `Info()` method on the application, which tells
us the number of transactions in our Merkle tree.

Now, since every command opens a new connection, we provide the
`abci-cli console` and `abci-cli batch` commands, to allow multiple ABCI
messages to be sent over a single connection.

Running `abci-cli console` should drop you in an interactive console for
speaking ABCI messages to your application.

Try running these commands:

```
> echo hello
-> code: OK
-> data: hello
-> data.hex: 0x68656C6C6F

> info
-> code: OK
-> data: {"size":0}
-> data.hex: 0x7B2273697A65223A307D

> commit
-> code: OK
-> data.hex: 0x0000000000000000

> deliver_tx "abc"
-> code: OK

> info
-> code: OK
-> data: {"size":1}
-> data.hex: 0x7B2273697A65223A317D

> commit
-> code: OK
-> data.hex: 0x0200000000000000

> query "abc"
-> code: OK
-> log: exists
-> height: 0
-> value: abc
-> value.hex: 616263

> deliver_tx "def=xyz"
-> code: OK

> commit
-> code: OK
-> data.hex: 0x0400000000000000

> query "def"
-> code: OK
-> log: exists
-> height: 0
-> value: xyz
-> value.hex: 78797A
```

Note that if we do `deliver_tx "abc"` it will store `(abc, abc)`, but if
we do `deliver_tx "abc=efg"` it will store `(abc, efg)`.

Similarly, you could put the commands in a file and run
`abci-cli --verbose batch < myfile`.

## Counter - Another Example

Now that we've got the hang of it, let's try another application, the
"counter" app.

Like the kvstore app, its code can be found
[here](https://github.com/tendermint/tendermint/blob/master/abci/cmd/abci-cli/abci-cli.go)
and looks like:

```
func cmdCounter(cmd *cobra.Command, args []string) error {

    app := counter.NewCounterApplication(flagSerial)

    logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

    // Start the listener
    srv, err := server.NewServer(flagAddrC, flagAbci, app)
    if err != nil {
        return err
    }
    srv.SetLogger(logger.With("module", "abci-server"))
    if err := srv.Start(); err != nil {
        return err
    }

    // Wait forever
    cmn.TrapSignal(func() {
        // Cleanup
        srv.Stop()
    })
    return nil
}
```

The counter app doesn't use a Merkle tree, it just counts how many times
we've sent a transaction, asked for a hash, or committed the state. The
result of `commit` is just the number of transactions sent.

This application has two modes: `serial=off` and `serial=on`.

When `serial=on`, transactions must be a big-endian encoded incrementing
integer, starting at 0.

If `serial=off`, there are no restrictions on transactions.

We can toggle the value of `serial` using the `set_option` ABCI message.

When `serial=on`, some transactions are invalid. In a live blockchain,
transactions collect in memory before they are committed into blocks. To
avoid wasting resources on invalid transactions, ABCI provides the
`check_tx` message, which application developers can use to accept or
reject transactions, before they are stored in memory or gossipped to
other peers.

In this instance of the counter app, `check_tx` only allows transactions
whose integer is greater than the last committed one.

Let's kill the console and the kvstore application, and start the
counter app:

```
abci-cli counter
```

In another window, start the `abci-cli console`:

```
> set_option serial on
-> code: OK
-> log: OK (SetOption doesn't return anything.)

> check_tx 0x00
-> code: OK

> check_tx 0xff
-> code: OK

> deliver_tx 0x00
-> code: OK

> check_tx 0x00
-> code: BadNonce
-> log: Invalid nonce. Expected >= 1, got 0

> deliver_tx 0x01
-> code: OK

> deliver_tx 0x04
-> code: BadNonce
-> log: Invalid nonce. Expected 2, got 4

> info
-> code: OK
-> data: {"hashes":0,"txs":2}
-> data.hex: 0x7B22686173686573223A302C22747873223A327D
```

This is a very simple application, but between `counter` and `kvstore`,
its easy to see how you can build out arbitrary application states on
top of the ABCI. [Hyperledger's
Burrow](https://github.com/hyperledger/burrow) also runs atop ABCI,
bringing with it Ethereum-like accounts, the Ethereum virtual-machine,
Monax's permissioning scheme, and native contracts extensions.

But the ultimate flexibility comes from being able to write the
application easily in any language.

We have implemented the counter in a number of languages [see the
example directory](https://github.com/tendermint/tendermint/tree/develop/abci/example).

To run the Node JS version, `cd` to `example/js` and run

```
node app.js
```

(you'll have to kill the other counter application process). In another
window, run the console and those previous ABCI commands. You should get
the same results as for the Go version.

## Bounties

Want to write the counter app in your favorite language?! We'd be happy
to add you to our [ecosystem](https://tendermint.com/ecosystem)! We're
also offering [bounties](https://hackerone.com/tendermint/) for
implementations in new languages!

The `abci-cli` is designed strictly for testing and debugging. In a real
deployment, the role of sending messages is taken by Tendermint, which
connects to the app using three separate connections, each with its own
pattern of messages.

For more information, see the [application developers
guide](./app-development.md). For examples of running an ABCI app with
Tendermint, see the [getting started guide](./getting-started.md).
Next is the ABCI specification.
