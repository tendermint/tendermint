---
order: 8
---

# Block Structure

The tendermint consensus engine records all agreements by a
supermajority of nodes into a blockchain, which is replicated among all
nodes. This blockchain is accessible via various rpc endpoints, mainly
`/block?height=` to get the full block, as well as
`/blockchain?minHeight=_&maxHeight=_` to get a list of headers. But what
exactly is stored in these blocks?

The [specification](https://github.com/tendermint/spec/blob/953523c3cb99fdb8c8f7a2d21e3a99094279e9de/spec/blockchain/blockchain.md) contains a detailed description of each component - that's the best place to get started.

To dig deeper, check out the [types package documentation](https://godoc.org/github.com/tendermint/tendermint/types).
