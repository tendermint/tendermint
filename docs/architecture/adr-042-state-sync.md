# ADR 042: State Sync Design

## Changelog

2019-06-27: Init by EB

## Context

"State Sync" would allow new nodes to skip downloading all the blocks and
just download a recent application state. Nodes would first perform a
light-client sync to find a recent height for which they could sync the state.
Then they would request verifiable pieces of that state concurrently from their peers.
Once they have a complete state, they can use "Block Sync" to download
and execute any more recent blocks.

Note that Tendermint does not actually know anything about the application
state; it is only available over ABCI. State Sync will thus require new ABCI
methods and/or connections, and will need to support arbitrary Merkle trees.

### Prior Work

There has been some related work in industry, though with different security
model and architecture. TODO.

Parity Ethereum implements a ["Warp
Sync"](https://wiki.parity.io/Warp-Sync-Snapshot-Format.html) to rapidly
download both blocks and state snapshots from peers. Data is carved into ~4MB
chunks and snappy compressed. Hashes of snappy compressed chunks are stored in a
manifest file which co-ordinates the state-sync. Obtaining a correct manifest
file seems to require an honest majority of peers. This means you may not find
out the state is incorrect until you download the whole thing and compare it
with a verified block header.

A similar solution was implemented by Binance in
[#3594](https://github.com/tendermint/tendermint/pull/3594)
based on their initial implementation in
[PR #3243](https://github.com/tendermint/tendermint/pull/3243)
and [some learnings](https://docs.google.com/document/d/1npGTAa1qxe8EQZ1wG0a0Sip9t5oX2vYZNUDwr_LVRR4/edit).
Note this still requires the honest majority peer assumption.
One major advantage of the warp-sync approach is that chunks can be compressed before sending.

Ideally State Sync in Tendermint would not require an honest majority of peers, and instead
would rely on the standard light client security, meaning state chunks would be
verifiable against the merkle root of the state. Such a design for tendermint
was originally tracked in [#828](https://github.com/tendermint/tendermint/issues/828).

An [initial specification](https://docs.google.com/document/d/15MFsQtNA0MGBv7F096FFWRDzQ1vR6_dics5Y49vF8JU/edit?ts=5a0f3629) was published by Alexis Sellier.
In this design, the state has a given `size` of primitive elements (like keys or
nodes), each element is assigned a number from 0 to `size-1`,
and chunks consists of a range of such elements.
Ackratos raised
[some concerns](https://docs.google.com/document/d/1npGTAa1qxe8EQZ1wG0a0Sip9t5oX2vYZNUDwr_LVRR4/edit)
about this design, somewhat specific to the IAVL tree, and mainly concerning
performance of random reads and of iterating through the tree to determine element numbers
(ie. elements aren't indexed by the element number).

This design could be generalized to have numbered chunks instead of elements,
where chunks are indexed by their chunk number and determined by some algorithm
for grouping the elements in the tree. This would allow pre-computation and
persistence of the chunks that could alleviate the concerns about real-time iteration and random reads.
However, there seems to be a larger issue with the "numbered chunks" approach:
it does not seem possible, in general, to verify that the chunk corresponds
to the chunk number, even if it can be verified that the chunk is a valid set of
nodes in the tree. A malicious peer could provide a chunk that contains valid
nodes from the tree but does not actually match the alleged chunk number. This
could make it difficult for nodes to figure out which parts of the tree they
have and which parts they still need. Hence, to go with this chunking approach
to state sync, information about how state is chunked would need to be hashed
into the state itself, or into the block header, so that chunk numbers could be
directly verified.

An alternative design was suggested by Jae Kwon in
[#3639](https://github.com/tendermint/tendermint/issues/3639) where chunking
happens lazily and in a dynamic way: nodes request key ranges from their peers,
and peers respond with some subset of the
requested range and with notes on how to request the rest in parallel from other
peers. Unlike chunk numbers, keys can be verified directly. And if some keys in the
range are ommitted, proofs for the range will fail to verify.
This way a node can start by requesting the entire tree from one peer,
and that peer can respond with say the first few keys, and the ranges to request
from other peers.

### Considerations

- Should we also support a warp-sync version? It would be faster
  and maybe easier for tree implementers, but less secure (relies on
  honest-majority of peers, rather than light-client)
- Should we include info about state sync into the state itself so we can use
  numbered chunks instead of key ranges? It would make the protocol simpler and
  faster, but at the cost of a large precompute step and some more coupling with
  the state or block header.

### Next Steps

To move forward with a light-client secure state sync based on key-ranges, here
are some next steps:

- The proposal in issue #3639 should be fleshed out and written up more formally as an ADR (ie.
  in this one)
- Need to address the initial light client sync (this will depend on a working
  light client!)
- Need to address assembling the Tendermint `State` object
- Need to determine how IAVL code will be updated to handle both reads and
  writes for state sync
- Need to address possible performance issues with iterating through the IAVL
  tree


