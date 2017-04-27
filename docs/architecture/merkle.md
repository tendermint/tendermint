# Merkle data stores

To allow the efficient creation of an ABCi app, tendermint wishes to provide a reference implemention of a key-value store that provides merkle proofs of the data.  These proofs then quickly allow the ABCi app to provide an apphash to the consensus engine, as well as a full proof to any client.

This engine is currently implemented in `go-merkle` with `merkleeyes` providing a language-agnostic binding via ABCi.  It uses `tmlibs/db` bindings internally to persist data to leveldb.

What are some of the requirements of this store:

* It must support efficient key-value operations (get/set/delete)
* It must support persistance.
* We must only persist complete blocks, so when we come up after a crash we are at the state of block N or N+1, but not in-between these two states.
* It must allow us to read/write from one uncommited state (working state), while serving other queries from the last commited state. And a way to determine which one to serve for each client.
* It must allow us to hold references to old state, to allow providing proofs from 20 blocks ago.  We can define some limits as to the maximum time to hold this data.
* We provide in process binding in Go
* We provide language-agnostic bindings when running the data store as it's own process.


