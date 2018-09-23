NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

For benchmarking tooling, see the `tm-bench` tools at https://github.com/tendermint/tools

**Q: How many txs/sec can Tendermint process?**

This depends on what kind of data structure holds the account data, what the persistence strategy is, how many accounts there are, and how many transactions there are in a block, and other factors.

Keeping 1M accounts *in memory* with the existing IAVLTree implementation (self-balancing key-value tree) with no Merkle hashing yields about 42000 get/set ops per second.

```bash
$ go test -bench=. merkle/*.go
  100000	     23564 ns/op
```

If you're sending funds from one account to another, you'll need 4 ops, so maybe that's 10500 txs/s. This does not take into account the overhead needed to process signatures or run EVM bytecode. Merkle hashing would be performed in batch between blocks, in this case. Storage could run in the background.

We still need to benchmark the mempool. The mempool is "safe" but it's slow. We'll look into overlaying a faster mempool network later when we know that it's the bottleneck.