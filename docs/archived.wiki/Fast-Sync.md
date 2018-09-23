NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

# Background

In a proof of work blockchain, syncing with the chain is the same process as staying up-to-date with the consensus: download blocks, and look for the one with the most total work. In proof-of-stake, the consensus process is more complex, as it involves rounds of communication between the nodes to determine what block should be committed next. Using this process to sync up with the blockchain from scratch can take a very long time. It's much faster to just download blocks and check the merkle tree of validators than to run the real-time consensus gossip protocol. 

# Fast Sync

To support faster syncing, tendermint offers a `fast-sync` mode, which is enabled by default, and can be toggled in the `config.toml` or via `--fast_sync=false`. 

In this mode, the tendermint daemon will sync hundreds of times faster than if it used the real-time consensus process. Once caught up, the daemon will switch out of fast sync and into the normal consensus mode. After running for some time, the node is considered `caught up` if it has at least one peer and it's height is at least as high as the max reported peer height. See [the IsCaughtUp method](https://github.com/tendermint/tendermint/blob/381fe19335ba8825e04d1d0fefa5cac709bb7178/blockchain/pool.go#L143).

If we're lagging sufficiently, we should go back to fast syncing, but this is an open issue: https://github.com/tendermint/tendermint/issues/129