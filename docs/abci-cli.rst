Using ABCI-CLI
==============

To facilitate testing and debugging of ABCI servers and simple apps, we
built a CLI, the ``abci-cli``, for sending ABCI messages from the
command line.

Install
-------

Make sure you `have Go installed <https://golang.org/doc/install>`__.

Next, install the ``abci-cli`` tool and example applications:

::

    go get -u github.com/tendermint/abci/cmd/...

If this fails, you may need to use ``glide`` to get vendored
dependencies:

::

    go get github.com/Masterminds/glide
    cd $GOPATH/src/github.com/tendermint/abci
    glide install
    go install ./cmd/...

Now run ``abci-cli --help`` to see the list of commands:

::

    COMMANDS:
       batch        Run a batch of ABCI commands against an application
       console      Start an interactive console for multiple commands
       echo         Have the application echo a message
       info         Get some info about the application
       set_option   Set an option on the application
       deliver_tx    Append a new tx to application
       check_tx     Validate a tx
       commit       Get application Merkle root hash
       help, h      Shows a list of commands or help for one command

    GLOBAL OPTIONS:
       --address "tcp://127.0.0.1:46658"    address of application socket
       --help, -h                           show help
       --version, -v                        print the version

Dummy - First Example
---------------------

The ``abci-cli`` tool lets us send ABCI messages to our application, to
help build and debug them.

The most important messages are ``deliver_tx``, ``check_tx``, and
``commit``, but there are others for convenience, configuration, and
information purposes.

Let's start a dummy application, which was installed at the same time as
``abci-cli`` above. The dummy just stores transactions in a merkle tree:

::

    dummy

In another terminal, run

::

    abci-cli echo hello
    abci-cli info

The application should echo ``hello`` and give you some information
about itself.

An ABCI application must provide two things:

-  a socket server
-  a handler for ABCI messages

When we run the ``abci-cli`` tool we open a new connection to the
application's socket server, send the given ABCI message, and wait for a
response.

The server may be generic for a particular language, and we provide a
`reference implementation in
Golang <https://github.com/tendermint/abci/tree/master/server>`__. See
the `list of other ABCI
implementations <https://tendermint.com/ecosystem>`__ for servers in
other languages.

The handler is specific to the application, and may be arbitrary, so
long as it is deterministic and conforms to the ABCI interface
specification.

So when we run ``abci-cli info``, we open a new connection to the ABCI
server, which calls the ``Info()`` method on the application, which
tells us the number of transactions in our Merkle tree.

Now, since every command opens a new connection, we provide the
``abci-cli console`` and ``abci-cli batch`` commands, to allow multiple
ABCI messages to be sent over a single connection.

Running ``abci-cli console`` should drop you in an interactive console
for speaking ABCI messages to your application.

Try running these commands:

::

    > echo hello
    -> data: hello

    > info
    -> data: {"size":0}

    > commit
    -> data: 0x

    > deliver_tx "abc"
    -> code: OK

    > info
    -> data: {"size":1}

    > commit
    -> data: 0x750502FC7E84BBD788ED589624F06CFA871845D1

    > query "abc"
    -> code: OK
    -> data: {"index":0,"value":"abc","exists":true}

    > deliver_tx "def=xyz"
    -> code: OK

    > commit
    -> data: 0x76393B8A182E450286B0694C629ECB51B286EFD5

    > query "def"
    -> code: OK
    -> data: {"index":1,"value":"xyz","exists":true}

Note that if we do ``deliver_tx "abc"`` it will store ``(abc, abc)``,
but if we do ``deliver_tx "abc=efg"`` it will store ``(abc, efg)``.

Similarly, you could put the commands in a file and run
``abci-cli --verbose batch < myfile``.

Counter - Another Example
-------------------------

Now that we've got the hang of it, let's try another application, the
"counter" app.

The counter app doesn't use a Merkle tree, it just counts how many times
we've sent a transaction, asked for a hash, or committed the state. The
result of ``commit`` is just the number of transactions sent.

This application has two modes: ``serial=off`` and ``serial=on``.

When ``serial=on``, transactions must be a big-endian encoded
incrementing integer, starting at 0.

If ``serial=off``, there are no restrictions on transactions.

We can toggle the value of ``serial`` using the ``set_option`` ABCI
message.

When ``serial=on``, some transactions are invalid. In a live blockchain,
transactions collect in memory before they are committed into blocks. To
avoid wasting resources on invalid transactions, ABCI provides the
``check_tx`` message, which application developers can use to accept or
reject transactions, before they are stored in memory or gossipped to
other peers.

In this instance of the counter app, ``check_tx`` only allows
transactions whose integer is greater than the last committed one.

Let's kill the console and the dummy application, and start the counter
app:

::

    counter

In another window, start the ``abci-cli console``:

::

    > set_option serial on
    -> data: serial=on

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
    -> data: {"hashes":0,"txs":2}

This is a very simple application, but between ``counter`` and
``dummy``, its easy to see how you can build out arbitrary application
states on top of the ABCI. `Hyperledger's
Burrow <https://github.com/hyperledger/burrow>`__ also runs atop ABCI,
bringing with it Ethereum-like accounts, the Ethereum virtual-machine,
Monax's permissioning scheme, and native contracts extensions.

But the ultimate flexibility comes from being able to write the
application easily in any language.

We have implemented the counter in a number of languages (see the
example directory).

To run the Node JS version, ``cd`` to ``example/js`` and run

::

    node app.js

(you'll have to kill the other counter application process). In another
window, run the console and those previous ABCI commands. You should get
the same results as for the Go version.

Bounties
--------

Want to write the counter app in your favorite language?! We'd be happy
to add you to our `ecosystem <https://tendermint.com/ecosystem>`__!
We're also offering `bounties <https://tendermint.com/bounties>`__ for
implementations in new languages!

The ``abci-cli`` is designed strictly for testing and debugging. In a
real deployment, the role of sending messages is taken by Tendermint,
which connects to the app using three separate connections, each with
its own pattern of messages.

For more information, see the `application developers
guide <./app-development.html>`__. For examples of running an ABCI
app with Tendermint, see the `getting started
guide <./getting-started.html>`__.
