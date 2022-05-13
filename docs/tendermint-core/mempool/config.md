---
order: 2
---

# Configuration

Here we describe configuration options around mempool.
For the purposes of this document, they are described
in a toml file, but some of them can also be passed in as
environmental variables.

Config:

```toml
[mempool]

# Set true to broadcast transactions in the mempool to other nodes
broadcast = true

# Maximum number of transactions in the mempool
size = 5000

# Limit the total size of all txs in the mempool.
# This only accounts for raw transactions (e.g. given 1MB transactions and
# max-txs-bytes=5MB, mempool will only accept 5 transactions).
max-txs-bytes = 1073741824

# Size of the cache (used to filter transactions we saw earlier) in transactions
cache-size = 10000

# Do not remove invalid transactions from the cache (default: false)
# Set to true if it's not possible for any invalid transaction to become valid
# again in the future.
keep-invalid-txs-in-cache = false

# Maximum size of a single transaction.
# NOTE: the max size of a tx transmitted over the network is {max-tx-bytes}.
max-tx-bytes = 1048576

# Maximum size of a batch of transactions to send to a peer
# Including space needed by encoding (one varint per transaction).
# XXX: Unused due to https://github.com/tendermint/tendermint/issues/5796
max-batch-bytes = 0
```

## Broadcast

Determines whether this node gossips any valid transactions
that arrive in mempool. Default is to gossip anything that
passes checktx. If this is disabled, transactions are not
gossiped, but instead stored locally and added to the next
block this node is the proposer.

## WalDir

This defines the directory where mempool writes the write-ahead
logs. These files can be used to reload unbroadcasted
transactions if the node crashes.

If the directory passed in is an absolute path, the wal file is
created there. If the directory is a relative path, the path is
appended to home directory of the tendermint process to
generate an absolute path to the wal directory
(default `$HOME/.tendermint` or set via `TM_HOME` or `--home`)

## Size

Size defines the total amount of transactions stored in the mempool. Default is `5_000` but can be adjusted to any number you would like. The higher the size the more strain on the node.

## Max Transactions Bytes

Max transactions bytes defines the total size of all the transactions in the mempool. Default is 1 GB.

## Cache size

Cache size determines the size of the cache holding transactions we have already seen. The cache exists to avoid running `checktx` each time we receive a transaction.

## Keep Invalid Transactions In Cache

Keep invalid transactions in cache determines wether a transaction in the cache, which is invalid, should be evicted. An invalid transaction here may mean that the transaction may rely on a different tx that has not been included in a block.

## Max Transaction Bytes

Max transaction bytes defines the max size a transaction can be for your node. If you would like your node to only keep track of smaller transactions this field would need to be changed. Default is 1MB.

## Max Batch Bytes

Max batch bytes defines the amount of bytes the node will send to a peer. Default is 0.

> Note: Unused due to https://github.com/tendermint/tendermint/issues/5796
