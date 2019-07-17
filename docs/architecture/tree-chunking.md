# Tree Chunking

In order to securely sync a Merkle tree, we must have some algorithm for
splitting it into chunks that remain verifiably part of the Merkle tree.
Whether the chunks are computed eagerly ahead of time,
or lazily in real time, is an orthogonal consideration.

We refer to the node computing the chunks from a complete Merkle tree as the
"chunker", and the node reassembling the Merkle tree from received chunks as the
"chunkee".

The main issue with various chunking strategies is how to verify that a given
chunk corresponds to a given chunk index. Especially since we desire for
chunkees to be able to apply chunks in any order, we need to ensure that
applying an out-of-order chunk is fully verifiable - ie. the chunk is a valid
part of the Merkle tree, and correctly corresponds to its chunk index.
Turns out this may not be possible in general.

## Arbitrary Chunking

There are two general approaches here - breadth-first and depth-first.
In the former case, we descend from the root one layer at a time, accumulating
full internal nodes into chunks. In the later case, we traverse the tree from
say left to right, accumulting full key-value pairs into chunks. In each case,
we require additional proof data, which consists of internal nodes that prove
all compnents of a chunk (whether nodes or key-value pairs) are valid members of
the tree.

Note that for AVL trees, which are insertion-order dependent, that the
depth-first approach must include all internal nodes in order to properly
communicate the structure of the tree, which cannot be derived from key-value
pairs alone. Such nodes would be part of the proof structure.

The chunker can assign to each chunk a unique index, and every chunker will
compute the same index for each chunk, assuming they follow the same
strategy. However, it is not clear that, in general, a chunkee can verify the
chunk index, as there is no explicit map from chunk index to chunk or vice
versa.

This raises two concerns for a state sync protocol:
- how to handle mis-indexed chunks (ie. requesting chunk 3 and receiving chunk
  4)
- how to handle non-indexed chunks (ie. receiving a chunk that doesn't
  correspond to any index from the given chunker strategy

In each case, it appears that liveness is made much more difficult by the need
to figure out what went wrong with the applied chunks. However, these problems
can be eliminated by requiring chunks to be applied in-order.

### MisIndexed Chunks

Suppose an honest chunker follows a strategy creating chunks 1,2,3,4,5.
Suppose an honest chunkee requests chunk 3 from a malicious chunker,
who responds with chunk 4. The chunkee applies chunk 4 to its Merkle tree
and verifies that it's correct. At this point the chunkee believes it
successfully applied chunk 3. This raises two questions:

- When the chunkee actually requests and receives chunk 4, and then discovers it already
  applied that chunk, how will it know which peer was malicious, the original
  one or the new one?
- How will the chunkee discover that it is missing the real chunk 3?

### NonIndexed Chunks

Suppose an honest chunker follows a strategy creating chunks 1,2,3,4,5.
Suppose a malicious chunker follows an alternative strategy, creating chunks
Q,R,S,T,U, none of which correspond to chunks 1,2,3,4,5, but all of which are
valid chunks in the tree.
Suppose an honest chunkee requests chunk 3 from the malicious chunker,
who responds with chunk R. The chunkee applies chunk R to its Merkle tree and
verifies that it's correct. At this point the chunkee believes it
successfully applied chunk 3. This raises two questions:

- When the chunkee receives another chunk that overlaps with chunk R, how will
  it know which peer was malicious, the original one or the new one?
- How will the chunkee determine which real chunks it is missing?

## Provably Indexed Chunking

An alternative approach would be a strategy that chunks the tree in a manner
such that all chunks have a verifiable index. Certain tree structures may be
able to incorporate this requirement into their design. In the extreme case,
the map from chunks to indices (or vice versa) could be committed into the tree
itself. But generally, we would like to avoid such requirements on the tree and
state and instead pursue a general strategy for chunking trees that preserves
verifiable chunk indices.

Consider an example strategy for illustration of how this would work:

- the first chunk (chunk 0) contains all nodes in the top 10 layers of the tree
- the nodes in the layer 10, which are part of chunk 0, form the roots for 1024
  sub-trees that contain the rest of the tree
- each of these sub-trees is its own chunk, numbered from 1 to 1024
- so long as chunk 0 is received first, each additional chunk can be verified
  with its index. For instance, if we receive chunk 5, the root hash of the
  sub-tree contained therein should correspond to the 5th node in layer 10

While this example provides the intuition for how the design might work,
we now need to generalize it to handle arbitrary tree sizes and bounded
chunk sizes. With the example above, we were only required to receive chunk 0 up
front, and could then receive the other chunks in any order. However to
generalize, we may be required to receive multiple chunks in order first, before
we get to a point where we can receive them in any order.

It may be the case that we need to receive all chunks in order until we get to a
depth where the remaining chunks are complete sub-trees and can be received in
any order. It is not clear how the chunkee can determine for itself when it
reaches this state; while it can verify it has all nodes up to a certain depth,
it does not necessarily know that each remaining sub-tree can fit in a chunk.

Thus it may not be possible to achieve this sort of out-of-order chunking.

## Conclusion

The above considerations demonstrate the challenges with applying chunks of a
Merkle tree out-of-order. While it may be possible with more complex protocol
design, it may be worth first considering a protocol that proceeds in-order, as
this should dramatically reduce the protocol complexity while maintaining the
ability to verify each chunk fully. This would also be very similar to the
blockchain reactor, which must process blocks in serial. Since any form of state sync
is likely to be orders of magnitude faster than the blockchain sync, the
performance loss from processing chunks in order is likely to be relatively small
compared to a more optimized, out-of-order approach.
