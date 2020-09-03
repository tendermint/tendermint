---
order: 10
---

# Fast Sync

In a proof of work blockchain, syncing with the chain is the same
process as staying up-to-date with the consensus: download blocks, and
look for the one with the most total work. In proof-of-stake, the
consensus process is more complex, as it involves rounds of
communication between the nodes to determine what block should be
committed next. Using this process to sync up with the blockchain from
scratch can take a very long time. It's much faster to just download
blocks and check the merkle tree of validators than to run the real-time
consensus gossip protocol.

## Using Fast Sync

To support faster syncing, Tendermint offers a `fast-sync` mode, which
is enabled by default, and can be toggled in the `config.toml` or via
`--fast_sync=false`.

In this mode, the Tendermint daemon will sync hundreds of times faster
than if it used the real-time consensus process. Once caught up, the
daemon will switch out of fast sync and into the normal consensus mode.
After running for some time, the node is considered `caught up` if it
has at least one peer and it's height is at least as high as the max
reported peer height. See [the IsCaughtUp
method](https://github.com/tendermint/tendermint/blob/b467515719e686e4678e6da4e102f32a491b85a0/blockchain/pool.go#L128).

Note: There are three versions of fast sync. We recommend using v0 as v1 and v2 are still in beta. 
  If you would like to use a different version you can do so by changing the version in the `config.toml`:

```toml
#######################################################
###       Fast Sync Configuration Connections       ###
#######################################################
[fastsync]

# Fast Sync version to use:
#   1) "v0" (default) - the legacy fast sync implementation
#   2) "v1" - refactor of v0 version for better testability
#   2) "v2" - complete redesign of v0, optimized for testability & readability 
version = "v0"
```

If we're lagging sufficiently, we should go back to fast syncing, but
this is an [open issue](https://github.com/tendermint/tendermint/issues/129).
