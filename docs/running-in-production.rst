Running in production
=====================

Logging
-------

Default logging level (``main:info,state:info,*:``) should suffice for normal
operation mode. Read `this post
<https://blog.cosmos.network/one-of-the-exciting-new-features-in-0-10-0-release-is-smart-log-level-flag-e2506b4ab756>__`
for details on how to configure ``log_level`` config variable. Some of the
modules can be found `here <./how-to-read-logs.html#list-of-modules>__`.

If you're trying to debug Tendermint or asked to provide logs with debug
logging level, you can do so by running tendermint with
``--log_level="*:debug"``.

Consensus WAL
-------------

Consensus module writes every message to the WAL (write-ahead log).

It also issues fsync syscall through `File#Sync
<https://golang.org/pkg/os/#File.Sync>__` for messages signed by this node (to
prevent double signing).

Under the hood, it uses `autofile.Group
<https://godoc.org/github.com/tendermint/tmlibs/autofile#Group>__`, which
rotates files when those get too big (> 10MB).

The total maximum size is 1GB. We only need the latest block and the block before it,
but if the former is dragging on across many rounds, we want all those rounds.

Replay
~~~~~~

Consensus module will replay all the messages of the last height written to WAL
before a crash (if such occurs).

The private validator may try to sign messages during replay because it runs
somewhat autonomously and does not know about replay process.

For example, if we got all the way to precommit in the WAL and then crash,
after we replay the proposal message, the private validator will try to sign a
prevote. But it will fail. That's ok because we’ll see the prevote later in the
WAL. Then it will go to precommit, and that time it will work because the
private validator contains the ``LastSignBytes`` and then we’ll replay the
precommit from the WAL.

Make sure to read about `WAL corruption
<./specification/corruption.html#wal-corruption>__` and recovery strategies.

DOS Exposure and Mitigation
---------------------------

Validators are supposed to setup `Sentry Node Architecture
<https://blog.cosmos.network/tendermint-explained-bringing-bft-based-pos-to-the-public-blockchain-domain-f22e274a0fdb>__`
to prevent Denial-of-service attacks. You can read more about it `here
<https://github.com/tendermint/aib-data/blob/develop/medium/TendermintBFT.md>__`.

Blockchain Reactor
~~~~~~~~~~~~~~~~~~

Defines ``maxMsgSize`` for the maximum size of incoming messages,
``SendQueueCapacity`` and ``RecvBufferCapacity`` for maximum sending and
receiving buffers respectively. These are supposed to prevent amplification
attacks by setting up the upper limit on how much data we can receive & send to
a peer.

Sending incorrectly encoded data will result in stopping the peer.

Consensus Reactor
~~~~~~~~~~~~~~~~~

Defines 4 channels: state, data, vote and vote_set_bits. Each channel
has  ``SendQueueCapacity`` and ``RecvBufferCapacity`` and
``RecvMessageCapacity`` set to ``maxMsgSize``.

Sending incorrectly encoded data will result in stopping the peer.

Evidence Reactor
~~~~~~~~~~~~~~~~

`#1503 <https://github.com/tendermint/tendermint/issues/1503>__`

Sending invalid evidence will result in stopping the peer.

Sending incorrectly encoded data or data exceeding ``maxMsgSize`` will result
in stopping the peer.

PEX Reactor
~~~~~~~~~~~

Defines only ``SendQueueCapacity``. `#1503 <https://github.com/tendermint/tendermint/issues/1503>__`

Implements rate-limiting by enforcing minimal time between two consecutive
``pexRequestMessage`` requests. If the peer sends us addresses we did not ask,
it is stopped.

Sending incorrectly encoded data or data exceeding ``maxMsgSize`` will result
in stopping the peer.

Mempool Reactor
~~~~~~~~~~~~~~~

`#1503 <https://github.com/tendermint/tendermint/issues/1503>__`

Mempool maintains a cache of the last 10000 transactions to prevent replaying
old transactions (plus transactions coming from other validators, who are
continually exchanging transactions). Read `Replay Protection
<./app-development.html#replay-protection>` for details.

Sending incorrectly encoded data or data exceeding ``maxMsgSize`` will result
in stopping the peer.

P2P
~~~

The core of the Tendermint peer-to-peer system is ``MConnection``. Each
connection has ``MaxPacketMsgPayloadSize``, which is the maximum packet size
and bounded send & receive queues. One can impose restrictions on send &
receive rate per connection (``SendRate``, ``RecvRate``).

RPC
~~~

Endpoints returning multiple entries are limited by default to return 30
elements (100 max).

Rate-limiting and authentication are another key aspects to help protect
against DOS attacks. While in the future we may implement these features, for
now, validators are supposed to use external tools like `NGINX
<https://www.nginx.com/blog/rate-limiting-nginx/>__` or `traefik
<https://docs.traefik.io/configuration/commons/#rate-limiting>__` to archive
the same things.
