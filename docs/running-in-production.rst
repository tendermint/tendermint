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
