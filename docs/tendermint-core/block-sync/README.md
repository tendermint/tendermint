---
order: 1
parent:
  title: Block Sync
  order: 6
---


# Block Sync
*Formerly known as Fast Sync*

In a proof of work blockchain, syncing with the chain is the same
process as staying up-to-date with the consensus: download blocks, and
look for the one with the most total work. In proof-of-stake, the
consensus process is more complex, as it involves rounds of
communication between the nodes to determine what block should be
committed next. Using this process to sync up with the blockchain from
scratch can take a very long time. It's much faster to just download
blocks and check the merkle tree of validators than to run the real-time
consensus gossip protocol.

## Using Block Sync

To support faster syncing, Tendermint offers a `blocksync` mode, which
is enabled by default, and can be toggled in the `config.toml` or via
`--blocksync.enable=false`.

In this mode, the Tendermint daemon will sync hundreds of times faster
than if it used the real-time consensus process. Once caught up, the
daemon will switch out of Block Sync and into the normal consensus mode.
After running for some time, the node is considered `caught up` if it
has at least one peer and it's height is at least as high as the max
reported peer height. See [the IsCaughtUp
method](https://github.com/tendermint/tendermint/blob/b467515719e686e4678e6da4e102f32a491b85a0/blockchain/pool.go#L128).

Note: There are multiple versions of Block Sync. Please use v0 as the other versions are no longer supported.
  If you would like to use a different version you can do so by changing the version in the `config.toml`:

```toml
#######################################################
###       Block Sync Configuration Connections       ###
#######################################################
[blocksync]

# If this node is many blocks behind the tip of the chain, BlockSync
# allows them to catchup quickly by downloading blocks in parallel
# and verifying their commits
enable = true

# Block Sync version to use:
#   1) "v0" (default) - the standard Block Sync implementation
#   2) "v2" - DEPRECATED, please use v0
version = "v0"
```

If we're lagging sufficiently, we should go back to block syncing, but
this is an [open issue](https://github.com/tendermint/tendermint/issues/129).

## The Block Sync event
When the tendermint blockchain core launches, it might switch to the `block-sync`
mode to catch up the states to the current network best height. the core will emits
a fast-sync event to expose the current status and the sync height. Once it catched
the network best height, it will switches to the state sync mechanism and then emit
another event for exposing the fast-sync `complete` status and the state `height`.

The user can query the events by subscribing `EventQueryBlockSyncStatus`
Please check [types](https://pkg.go.dev/github.com/tendermint/tendermint/types?utm_source=godoc#pkg-constants) for the details.

## Implementation

To read more on the implamentation please see the [reactor doc](./reactor.md) and the [implementation doc](./implementation.md)
