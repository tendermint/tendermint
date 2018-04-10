Using Tendermint
================

This is a guide to using the ``tendermint`` program from the command
line. It assumes only that you have the ``tendermint`` binary installed
and have some rudimentary idea of what Tendermint and ABCI are.

You can see the help menu with ``tendermint --help``, and the version
number with ``tendermint version``.

Directory Root
--------------

The default directory for blockchain data is ``~/.tendermint``. Override
this by setting the ``TMHOME`` environment variable.

Initialize
----------

Initialize the root directory by running:

::

    tendermint init

This will create a new private key (``priv_validator.json``), and a
genesis file (``genesis.json``) containing the associated public key,
in ``$TMHOME/config``.
This is all that's necessary to run a local testnet with one validator.

For more elaborate initialization, see our `testnet deployment
tool <https://github.com/tendermint/tools/tree/master/mintnet-kubernetes>`__.

Run
---

To run a Tendermint node, use

::

    tendermint node

By default, Tendermint will try to connect to an ABCI application on
`127.0.0.1:46658 <127.0.0.1:46658>`__. If you have the ``kvstore`` ABCI
app installed, run it in another window. If you don't, kill Tendermint
and run an in-process version of the ``kvstore`` app:

::

    tendermint node --proxy_app=kvstore

After a few seconds you should see blocks start streaming in. Note that
blocks are produced regularly, even if there are no transactions. See *No Empty Blocks*, below, to modify this setting.

Tendermint supports in-process versions of the ``counter``, ``kvstore`` and ``nil``
apps that ship as examples in the `ABCI
repository <https://github.com/tendermint/abci>`__. It's easy to compile
your own app in-process with Tendermint if it's written in Go. If your
app is not written in Go, simply run it in another process, and use the
``--proxy_app`` flag to specify the address of the socket it is
listening on, for instance:

::

    tendermint node --proxy_app=/var/run/abci.sock

Transactions
------------

To send a transaction, use ``curl`` to make requests to the Tendermint
RPC server, for example:

::

    curl http://localhost:46657/broadcast_tx_commit?tx=\"abcd\"

For handling responses, we recommend you `install the jsonpp
tool <http://jmhodges.github.io/jsonpp/>`__ to pretty print the JSON.

We can see the chain's status at the ``/status`` end-point:

::

    curl http://localhost:46657/status | jsonpp

and the ``sync_info.latest_app_hash`` in particular:

::

    curl http://localhost:46657/status |  jsonpp | grep sync_info.latest_app_hash

Visit http://localhost:46657 in your browser to see the list of other
endpoints. Some take no arguments (like ``/status``), while others
specify the argument name and use ``_`` as a placeholder.

Formatting
~~~~~~~~~~

The following nuances when sending/formatting transactions should
be taken into account:

With ``GET``:

To send a UTF8 string byte array, quote the value of the tx pramater:

::

    curl 'http://localhost:46657/broadcast_tx_commit?tx="hello"'

which sends a 5 byte transaction: "h e l l o" [68 65 6c 6c 6f].

Note the URL must be wrapped with single quoes, else bash will ignore the double quotes.
To avoid the single quotes, escape the double quotes:

::

    curl http://localhost:46657/broadcast_tx_commit?tx=\"hello\"



Using a special character:

::

    curl 'http://localhost:46657/broadcast_tx_commit?tx="€5"'

sends a 4 byte transaction: "€5" (UTF8) [e2 82 ac 35].

To send as raw hex, omit quotes AND prefix the hex string with ``0x``:

::

    curl http://localhost:46657/broadcast_tx_commit?tx=0x01020304

which sends a 4 byte transaction: [01 02 03 04].

With ``POST`` (using ``json``), the raw hex must be ``base64`` encoded:

::

    curl --data-binary '{"jsonrpc":"2.0","id":"anything","method":"broadcast_tx_commit","params": {"tx": "AQIDBA=="}}' -H 'content-type:text/plain;' http://localhost:46657

which sends the same 4 byte transaction: [01 02 03 04].

Note that raw hex cannot be used in ``POST`` transactions.

Reset
-----

**WARNING: UNSAFE** Only do this in development and only if you can
afford to lose all blockchain data!

To reset a blockchain, stop the node, remove the ``~/.tendermint/data``
directory and run

::

    tendermint unsafe_reset_priv_validator

This final step is necessary to reset the ``priv_validator.json``, which
otherwise prevents you from making conflicting votes in the consensus
(something that could get you in trouble if you do it on a real
blockchain). If you don't reset the ``priv_validator.json``, your fresh
new blockchain will not make any blocks.

Configuration
-------------

Tendermint uses a ``config.toml`` for configuration. For details, see
`the config specification <./specification/configuration.html>`__.

Notable options include the socket address of the application
(``proxy_app``), the listening address of the Tendermint peer
(``p2p.laddr``), and the listening address of the RPC server
(``rpc.laddr``).

Some fields from the config file can be overwritten with flags.

No Empty Blocks
---------------

This much requested feature was implemented in version 0.10.3. While the
default behaviour of ``tendermint`` is still to create blocks approximately
once per second, it is possible to disable empty blocks or set a block creation
interval. In the former case, blocks will be created when there are new
transactions or when the AppHash changes.

To configure Tendermint to not produce empty blocks unless there are
transactions or the app hash changes, run Tendermint with this additional flag:

::

    tendermint node --consensus.create_empty_blocks=false

or set the configuration via the ``config.toml`` file:

::

    [consensus]
    create_empty_blocks = false

Remember: because the default is to *create empty blocks*, avoiding empty blocks requires the config option to be set to ``false``.

The block interval setting allows for a delay (in seconds) between the creation of each new empty block. It is set via the ``config.toml``:

::

    [consensus]
    create_empty_blocks_interval = 5

With this setting, empty blocks will be produced every 5s if no block has been produced otherwise,
regardless of the value of ``create_empty_blocks``.

Broadcast API
-------------

Earlier, we used the ``broadcast_tx_commit`` endpoint to send a
transaction. When a transaction is sent to a Tendermint node, it will
run via ``CheckTx`` against the application. If it passes ``CheckTx``,
it will be included in the mempool, broadcast to other peers, and
eventually included in a block.

Since there are multiple phases to processing a transaction, we offer
multiple endpoints to broadcast a transaction:

::

    /broadcast_tx_async
    /broadcast_tx_sync
    /broadcast_tx_commit

These correspond to no-processing, processing through the mempool, and
processing through a block, respectively. That is,
``broadcast_tx_async``, will return right away without waiting to hear
if the transaction is even valid, while ``broadcast_tx_sync`` will
return with the result of running the transaction through ``CheckTx``.
Using ``broadcast_tx_commit`` will wait until the transaction is
committed in a block or until some timeout is reached, but will return
right away if the transaction does not pass ``CheckTx``. The return
value for ``broadcast_tx_commit`` includes two fields, ``check_tx`` and
``deliver_tx``, pertaining to the result of running the transaction
through those ABCI messages.

The benefit of using ``broadcast_tx_commit`` is that the request returns
after the transaction is committed (i.e. included in a block), but that
can take on the order of a second. For a quick result, use
``broadcast_tx_sync``, but the transaction will not be committed until
later, and by that point its effect on the state may change.

Note: see the Transactions => Formatting section for details about
transaction formating.

Tendermint Networks
-------------------

When ``tendermint init`` is run, both a ``genesis.json`` and
``priv_validator.json`` are created in ``~/.tendermint/config``. The
``genesis.json`` might look like:

::

    {
        "app_hash": "",
        "chain_id": "test-chain-HZw6TB",
        "genesis_time": "0001-01-01T00:00:00.000Z",
        "validators": [
            {
                "power": 10,
                "name": "",
                "pub_key": [
                    1,
                    "5770B4DD55B3E08B7F5711C48B516347D8C33F47C30C226315D21AA64E0DFF2E"
                ]
            }
        ]
    }

And the ``priv_validator.json``:

::

    {
        "address": "4F4D895F882A18E1D1FC608D102601DA8D3570E5",
        "last_height": 0,
        "last_round": 0,
        "last_signature": null,
        "last_signbytes": "",
        "last_step": 0,
        "priv_key": [
            1,
            "F9FA3CD435BDAE54D0BCA8F1BC289D718C23D855C6DB21E8543F5E4F457E62805770B4DD55B3E08B7F5711C48B516347D8C33F47C30C226315D21AA64E0DFF2E"
        ],
        "pub_key": [
            1,
            "5770B4DD55B3E08B7F5711C48B516347D8C33F47C30C226315D21AA64E0DFF2E"
        ]
    }

The ``priv_validator.json`` actually contains a private key, and should
thus be kept absolutely secret; for now we work with the plain text.
Note the ``last_`` fields, which are used to prevent us from signing
conflicting messages.

Note also that the ``pub_key`` (the public key) in the
``priv_validator.json`` is also present in the ``genesis.json``.

The genesis file contains the list of public keys which may participate in the
consensus, and their corresponding voting power. Greater than 2/3 of the voting
power must be active (i.e. the corresponding private keys must be producing
signatures) for the consensus to make progress. In our case, the genesis file
contains the public key of our ``priv_validator.json``, so a Tendermint node
started with the default root directory will be able to make progress. Voting
power uses an `int64` but must be positive, thus the range is: 0 through
9223372036854775807. Because of how the current proposer selection algorithm works,
we do not recommend having voting powers greater than 10^12 (ie. 1 trillion)
(see `Proposals section of Byzantine Consensus Algorithm
<./specification/byzantine-consensus-algorithm.html#proposals>`__ for details).

If we want to add more nodes to the network, we have two choices: we can
add a new validator node, who will also participate in the consensus by
proposing blocks and voting on them, or we can add a new non-validator
node, who will not participate directly, but will verify and keep up
with the consensus protocol.

Peers
~~~~~

To connect to peers on start-up, specify them in the ``$TMHOME/config/config.toml`` or
on the command line. Use `seeds` to specify seed nodes from which you can get many other
peer addresses, and ``persistent_peers`` to specify peers that your node will maintain
persistent connections with.

For instance,

::

    tendermint node --p2p.seeds "f9baeaa15fedf5e1ef7448dd60f46c01f1a9e9c4@1.2.3.4:46656,0491d373a8e0fcf1023aaf18c51d6a1d0d4f31bd@5.6.7.8:46656"

Alternatively, you can use the ``/dial_seeds`` endpoint of the RPC to
specify seeds for a running node to connect to:

::

    curl 'localhost:46657/dial_seeds?seeds=\["f9baeaa15fedf5e1ef7448dd60f46c01f1a9e9c4@1.2.3.4:46656","0491d373a8e0fcf1023aaf18c51d6a1d0d4f31bd@5.6.7.8:46656"\]'

Note, if the peer-exchange protocol (PEX) is enabled (default), you should not
normally need seeds after the first start. Peers will be gossipping about known
peers and forming a network, storing peer addresses in the addrbook.

If you want Tendermint to connect to specific set of addresses and maintain a
persistent connection with each, you can use the ``--p2p.persistent_peers``
flag or the corresponding setting in the ``config.toml`` or the
``/dial_peers`` RPC endpoint to do it without stopping Tendermint
core instance.

::

    tendermint node --p2p.persistent_peers "429fcf25974313b95673f58d77eacdd434402665@10.11.12.13:46656,96663a3dd0d7b9d17d4c8211b191af259621c693@10.11.12.14:46656"
    curl 'localhost:46657/dial_peers?persistent=true&peers=\["429fcf25974313b95673f58d77eacdd434402665@10.11.12.13:46656","96663a3dd0d7b9d17d4c8211b191af259621c693@10.11.12.14:46656"\]'

Adding a Non-Validator
~~~~~~~~~~~~~~~~~~~~~~

Adding a non-validator is simple. Just copy the original
``genesis.json`` to ``~/.tendermint/config`` on the new machine and start the
node, specifying seeds or persistent peers as necessary. If no seeds or persistent
peers are specified, the node won't make any blocks, because it's not a validator,
and it won't hear about any blocks, because it's not connected to the other peer.

Adding a Validator
~~~~~~~~~~~~~~~~~~

The easiest way to add new validators is to do it in the
``genesis.json``, before starting the network. For instance, we could
make a new ``priv_validator.json``, and copy it's ``pub_key`` into the
above genesis.

We can generate a new ``priv_validator.json`` with the command:

::

    tendermint gen_validator

Now we can update our genesis file. For instance, if the new
``priv_validator.json`` looks like:

::

    {
            "address": "AC379688105901436A34A65F185C115B8BB277A1",
            "last_height": 0,
            "last_round": 0,
            "last_signature": null,
            "last_signbytes": "",
            "last_step": 0,
            "priv_key": [
                    1,
                    "0D2ED337D748ADF79BE28559B9E59EBE1ABBA0BAFE6D65FCB9797985329B950C8F2B5AACAACC9FCE41881349743B0CFDE190DF0177744568D4E82A18F0B7DF94"
            ],
            "pub_key": [
                    1,
                    "8F2B5AACAACC9FCE41881349743B0CFDE190DF0177744568D4E82A18F0B7DF94"
            ]
    }

then the new ``genesis.json`` will be:

::

    {
        "app_hash": "",
        "chain_id": "test-chain-HZw6TB",
        "genesis_time": "0001-01-01T00:00:00.000Z",
        "validators": [
            {
                "power": 10,
                "name": "",
                "pub_key": [
                    1,
                    "5770B4DD55B3E08B7F5711C48B516347D8C33F47C30C226315D21AA64E0DFF2E"
                ]
            },
            {
                "power": 10,
                "name": "",
                "pub_key": [
                    1,
                    "8F2B5AACAACC9FCE41881349743B0CFDE190DF0177744568D4E82A18F0B7DF94"
                ]
            }
        ]
    }

Update the ``genesis.json`` in ``~/.tendermint/config``. Copy the genesis file
and the new ``priv_validator.json`` to the ``~/.tendermint/config`` on a new
machine.

Now run ``tendermint node`` on both machines, and use either
``--p2p.persistent_peers`` or the ``/dial_peers`` to get them to peer up. They
should start making blocks, and will only continue to do so as long as
both of them are online.

To make a Tendermint network that can tolerate one of the validators
failing, you need at least four validator nodes (> 2/3).

Updating validators in a live network is supported but must be
explicitly programmed by the application developer. See the `application
developers guide <./app-development.html>`__ for more
details.

Local Network
~~~~~~~~~~~~~

To run a network locally, say on a single machine, you must change the
``_laddr`` fields in the ``config.toml`` (or using the flags) so that
the listening addresses of the various sockets don't conflict.
Additionally, you must set ``addrbook_strict=false`` in the
``config.toml``, otherwise Tendermint's p2p library will deny making
connections to peers with the same IP address.

Upgrading
~~~~~~~~~

The Tendermint development cycle includes a lot of breaking changes. Upgrading from
an old version to a new version usually means throwing away the chain data. Try out
the `tm-migrate <https://github.com/hxzqlh/tm-tools>`__ tool written by @hxqlh if
you are keen to preserve the state of your chain when upgrading to newer versions.
