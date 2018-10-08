# Mempool Configuration

Here we describe configuration options around mempool.
For the purposes of this document, they are described
as command-line flags, but they can also be passed in as
environmental variables or in the config.toml file. The
following are all equivalent:

Flag: `--mempool.recheck=false`

Environment: `TM_MEMPOOL_RECHECK=false`

Config:

```
[mempool]
recheck = false
```

## Recheck

`--mempool.recheck=false` (default: true)

Recheck determines if the mempool rechecks all pending
transactions after a block was committed. Once a block
is committed, the mempool removes all valid transactions
that were successfully included in the block.

If `recheck` is true, then it will rerun CheckTx on
all remaining transactions with the new block state.

## Broadcast

`--mempool.broadcast=false` (default: true)

Determines whether this node gossips any valid transactions
that arrive in mempool. Default is to gossip anything that
passes checktx. If this is disabled, transactions are not
gossiped, but instead stored locally and added to the next
block this node is the proposer.

## WalDir

`--mempool.wal_dir=/tmp/gaia/mempool.wal` (default: $TM_HOME/data/mempool.wal)

This defines the directory where mempool writes the write-ahead
logs. These files can be used to reload unbroadcasted
transactions if the node crashes.

If the directory passed in is an absolute path, the wal file is
created there. If the directory is a relative path, the path is
appended to home directory of the tendermint process to
generate an absolute path to the wal directory
(default `$HOME/.tendermint` or set via `TM_HOME` or `--home``)
