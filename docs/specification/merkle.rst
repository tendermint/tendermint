Merkle
======

For an overview of Merkle trees, see
`wikipedia <https://en.wikipedia.org/wiki/Merkle_tree>`__.

There are two types of Merkle trees used in Tendermint.

-  ```IAVL+ Tree`` <#iavl-tree>`__: An immutable self-balancing binary
   tree for persistent application state
-  ```Simple Tree`` <#simple-tree>`__: A simple compact binary tree for
   a static list of items

IAVL+ Tree
----------

The purpose of this data structure is to provide persistent storage for
key-value pairs (e.g. account state, name-registrar data, and
per-contract data) such that a deterministic merkle root hash can be
computed. The tree is balanced using a variant of the `AVL
algorithm <http://en.wikipedia.org/wiki/AVL_tree>`__ so all operations
are O(log(n)).

Nodes of this tree are immutable and indexed by its hash. Thus any node
serves as an immutable snapshot which lets us stage uncommitted
transactions from the mempool cheaply, and we can instantly roll back to
the last committed state to process transactions of a newly committed
block (which may not be the same set of transactions as those from the
mempool).

In an AVL tree, the heights of the two child subtrees of any node differ
by at most one. Whenever this condition is violated upon an update, the
tree is rebalanced by creating O(log(n)) new nodes that point to
unmodified nodes of the old tree. In the original AVL algorithm, inner
nodes can also hold key-value pairs. The AVL+ algorithm (note the plus)
modifies the AVL algorithm to keep all values on leaf nodes, while only
using branch-nodes to store keys. This simplifies the algorithm while
minimizing the size of merkle proofs

In Ethereum, the analog is the `Patricia
trie <http://en.wikipedia.org/wiki/Radix_tree>`__. There are tradeoffs.
Keys do not need to be hashed prior to insertion in IAVL+ trees, so this
provides faster iteration in the key space which may benefit some
applications. The logic is simpler to implement, requiring only two
types of nodes -- inner nodes and leaf nodes. The IAVL+ tree is a binary
tree, so merkle proofs are much shorter than the base 16 Patricia trie.
On the other hand, while IAVL+ trees provide a deterministic merkle root
hash, it depends on the order of updates. In practice this shouldn't be
a problem, since you can efficiently encode the tree structure when
serializing the tree contents.

Simple Tree
-----------

For merkelizing smaller static lists, use the Simple Tree. The
transactions and validation signatures of a block are hashed using this
simple merkle tree logic.

If the number of items is not a power of two, the tree will not be full
and some leaf nodes will be at different levels. Simple Tree tries to
keep both sides of the tree the same size, but the left side may be one
greater.

::

        Simple Tree with 6 items           Simple Tree with 7 items 
                                                             
                   *                                  *             
                  / \                                / \            
                /     \                            /     \          
              /         \                        /         \        
            /             \                    /             \      
           *               *                  *               *     
          / \             / \                / \             / \    
         /   \           /   \              /   \           /   \   
        /     \         /     \            /     \         /     \  
       *       h2      *       h5         *       *       *       h6
      / \             / \                / \     / \     / \        
     h0  h1          h3  h4             h0  h1  h2  h3  h4  h5      

Simple Tree with Dictionaries
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The Simple Tree is used to merkelize a list of items, so to merkelize a
(short) dictionary of key-value pairs, encode the dictionary as an
ordered list of ``KVPair`` structs. The block hash is such a hash
derived from all the fields of the block ``Header``. The state hash is
similarly derived.
