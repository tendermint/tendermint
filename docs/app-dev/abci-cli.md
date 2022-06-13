---
order: 2
---

# Using ABCI-CLI

To facilitate testing and debugging of ABCI servers and simple apps, we
built a CLI, the `abci-cli`, for sending ABCI messages from the command
line.

## Install

Make sure you [have Go installed](https://golang.org/doc/install).

Next, install the `abci-cli` tool and example applications:

```sh
git clone https://github.com/tendermint/tendermint.git
cd tendermint
make install_abci
```

Now run `abci-cli` to see the list of commands:

```sh
Usage:
  abci-cli [command]

Available Commands:
  batch          Run a batch of abci commands against an application
  check_tx       Validate a tx
  commit         Commit the application state and return the Merkle root hash
  console        Start an interactive abci console for multiple commands
  finalize_block Send a set of transactions to the application
  kvstore        ABCI demo example
  echo           Have the application echo a message
  help           Help about any command
  info           Get some info about the application
  query          Query the application state
  set_option     Set an options on the application

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

The most important messages are `finalize_block`, `check_tx`, and `commit`,
but there are others for convenience, configuration, and information
purposes.

We'll start a kvstore application, which was installed at the same time
as `abci-cli` above. The kvstore just stores transactions in a merkle
tree.

Its code can be found
[here](https://github.com/tendermint/tendermint/blob/master/abci/cmd/abci-cli/abci-cli.go)
and looks like:

```go
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

    // Stop upon receiving SIGTERM or CTRL-C.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

    srv.SetLogger(logger.With("module", "abci-server"))
    if err := srv.Start(ctx); err != nil {
        return err
    }

    // Run until shutdown.
<-ctx.Done()
srv.Wait()
}
```

Start by running:

```sh
abci-cli kvstore
```

And in another terminal, run

```sh
abci-cli echo hello
abci-cli info
```

You'll see something like:

```sh
-> data: hello
-> data.hex: 68656C6C6F
```

and:

```sh
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
Golang](https://github.com/tendermint/tendermint/tree/master/abci/server). See the
[list of other ABCI implementations](https://github.com/tendermint/awesome#ecosystem) for servers in
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

```sh
> echo hello
-> code: OK
-> data: hello
-> data.hex: 0x68656C6C6F

> info
-> code: OK
-> data: {"size":0}
-> data.hex: 0x7B2273697A65223A307D

> finalize_block "abc"
-> code: OK
-> code: OK
-> data.hex: 0x0200000000000000

> commit
-> code: OK

> info
-> code: OK
-> data: {"size":1}
-> data.hex: 0x7B2273697A65223A317D

> query "abc"
-> code: OK
-> log: exists
-> height: 1
-> key: abc
-> key.hex: 616263
-> value: abc
-> value.hex: 616263

> finalize_block "def=xyz" "ghi=123"
-> code: OK
-> code: OK
-> code: OK
-> data.hex: 0x0600000000000000

> commit
-> code: OK

> query "def"
-> code: OK
-> log: exists
-> height: 2
-> key: def
-> key.hex: 646566
-> value: xyz
-> value.hex: 78797A
```

Note that if we do `finalize_block "abc" ...` it will store `(abc, abc)`, but if
we do `finalize_block "abc=efg" ...` it will store `(abc, efg)`.

Similarly, you could put the commands in a file and run
`abci-cli --verbose batch < myfile`.

## Bounties

Want to write an app in your favorite language?! We'd be happy
to add you to our [ecosystem](https://github.com/tendermint/awesome#ecosystem)!
See [funding](https://github.com/interchainio/funding) opportunities from the
[Interchain Foundation](https://interchain.io/) for implementations in new languages and more.

The `abci-cli` is designed strictly for testing and debugging. In a real
deployment, the role of sending messages is taken by Tendermint, which
connects to the app using three separate connections, each with its own
pattern of messages.

For examples of running an ABCI app with
Tendermint, see the [getting started guide](./getting-started.md).
Next is the ABCI specification.
