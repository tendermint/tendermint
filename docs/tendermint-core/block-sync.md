---
order: 10
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

To support faster syncing, Tendermint offers a `block-sync` mode, which
is enabled by default, and can be toggled in the `config.toml` or via
`--block_sync=false`.

In this mode, the Tendermint daemon will sync hundreds of times faster
than if it used the real-time consensus process. Once caught up, the
daemon will switch out of Block Sync and into the normal consensus mode.
After running for some time, the node is considered `caught up` if it
has at least one peer and it's height is at least as high as the max
reported peer height. See [the IsCaughtUp
method](https://github.com/tendermint/tendermint/blob/b467515719e686e4678e6da4e102f32a491b85a0/blockchain/pool.go#L128).

Note: While there have historically been multiple versions of blocksync, v0, v1, and v2, all versions
other than v0 have been deprecated in favor of the simplest and most well understood algorithm.

```toml
#######################################################
###       Block Sync Configuration Connections       ###
#######################################################
[blocksync]

# Block Sync version to use:
#   1) "v0" (default) - the legacy fast sync implementation
#   All other versions have been deprecated and will error if used.
version = "v0"
```

If we're lagging sufficiently, we should go back to block syncing, but
this is an [open issue](https://github.com/tendermint/tendermint/issues/129).
