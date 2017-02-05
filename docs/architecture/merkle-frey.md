# Merkle data stores - Frey's proposal

## TL;DR

To allow the efficient creation of an ABCi app, tendermint wishes to provide a reference implementation of a key-value store that provides merkle proofs of the data.  These proofs then quickly allow the ABCi app to provide an app hash to the consensus engine, as well as a full proof to any client.

This is equivalent to building a database, and I would propose designing it from the API first, then looking how to implement this (or make an adapter from the API to existing implementations). Once we agree on the functionality and the interface, we can implement the API bindings, and then work on building adapters to existence merkle-ized data stores, or modifying the stores to support this interface.

We need to consider the API (both in-process and over the network), language bindings, maintaining handles to old state (and garbage collecting), persistence, security, providing merkle proofs, and general key-value store operations. To stay consistent with the blockchains "single global order of operations", this data store should only allow one connection at a time to have write access.

## Overview

* **State**
  * There are two concepts of state, "committed state" and "working state"
  * The working state is only accessible from the ABCi app, allows writing, but does not need to support proofs.
  * When we commit the "working state", it becomes a new "committed state" and has an immutable root hash, provides proofs, and can be exposed to external clients.
* **Transactions**
  * The database always allows creating a read-only transaction at the last "committed state", this transaction can serve read queries and proofs.
  * The database maintains all data to serve these read transactions until they are closed by the client (or time out).  This allows the client(s) to determine how much old info is needed
  * The database can only support *maximal* one writable transaction at a time.  This makes it easy to enforce serializability, and attempting to start a second writable transaction may trigger a panic.
* **Functionality**
  * It must support efficient key-value operations (get/set/delete)
  * It must support returning merkle proofs for any "committed state"
  * It should support range queries on subsets of the key space if possible (ie. if the db doesn't hash keys)
  * It should also support listening to changes to a desired key via pub-sub or similar method, so I can quickly notify you on a change to your balance without constant polling.
  * It may support other db-specific query types as an extension to this interface, as long as all specified actions maintain their meaning.
* **Interface**
  * This interface should be domain-specific - ie. designed just for this use case
  * It should present a simple go interface for embedding the data store in-process
  * It should create a gRPC/protobuf API for calling from any client
  * It should provide and maintain client adapters from our in-process interface to gRPC client calls for at least golang and Java (maybe more languages?)
  * It should provide and maintain server adapters from our gRPC calls to the in-process interface for golang at least (unless there is another server we wish to support)
* **Persistence**
  * It must support atomic persistence upon committing a new block.  That is, upon crash recovery, the state is guaranteed to represent the state at the end of a complete block (along with a note of which height it was).
  * It must delay deletion of old data as long as there are open read-only transactions referring to it, thus we must maintain some sort of WAL to keep track of pending cleanup.
  * When a transaction is closed, or when we recover from a crash, it should clean up all no longer needed data to avoid memory/storage leaks.
* **Security and Auth**
  * If we allow connections over gRPC, we must consider this issues and allow both encryption (SSL), and some basic auth rules to prevent undesired access to the DB
  * This is client-specific and does not need to be supported in the in-process, embedded version.

## Details

Here we go more in-depth in each of the sections, explaining the reasoning and more details on the desired behavior. This document is only the high-level architecture and should support multiple implementations.  When building out a specific implementation, a similar document should be provided for that repo, showing how it implements these concepts, and details about memory usage, storage, efficiency, etc.


### State

The current ABCi interface avoids this question a bit and that has brought confusion.  If I use `merkleeyes` to store data, which state is returned from `Query`?  The current "working" state, which I would like to refer to in my ABCi application?  Or the last committed state, which I would like to return to a client's query?  Or an old state, which I may select based on height?

Right now, `merkleeyes` implements `Query` like a normal ABCi app and only returns committed state, which has lead to problems and confusion.  Thus, we need to be explicit about which state we want to view.  Each viewer can then specify which state it wants to view.  This allows the app to query the working state in DeliverTx, but the committed state in Query.

We can easily provide two global references for "last committed" and "current working" states.  However, if we want to also allow querying of older commits... then we need some way to keep track of which ones are still in use, so we can garbage collect the unneeded ones. There is a non-trivial overhead in holding references to all past states, but also a hard-coded solution (hold onto the last 5 commits) may not support all clients.  We should let the client define this somehow.

### Transactions

Transactions (in the typical database sense) are a clean and established solution to this issue.  We can look at the [isolations levels](https://en.wikipedia.org/wiki/Isolation_(database_systems)#Serializable) which attempt to provide us things like "repeatable reads".  That means if we open a transaction, and query some data 100 times while other processes are writing to the db, we get the same result each time.  This transaction has a reference to its own local state from the time the transaction started. (We are referring to the highest isolation levels here, which correlate well this the blockchain use case).

If we implement a read-only transaction as a reference to state at the time of creation of that transaction, we can then hold these references to various snapshots, one per block that we are interested, and allow the client to multiplex queries and proofs from these various blocks.

If we continue using these concepts (which have informed 30+ years of server side design), we can add a few nice features to our write transactions. The first of which is `Rollback` and `Commit`.  That means all the changes we make in this transaction have no effect on the database until they are committed.  And until they are committed, we can always abort if we detect an anomaly, returning to the last committed state with a rollback.

There is also a nice extension to this available on some database servers, basically, "nested" transactions or "savepoints".  This means that within one transaction, you can open a subtransaction/savepoint and continue work.  Later you have the option to commit or rollback all work since the savepoint/subtransaction.  And then continue with the main transaction.

If you don't understand why this is useful, look at how basecoin needs to [hold cached state for AppTx](https://github.com/tendermint/basecoin/blob/master/state/execution.go#L126-L149), meaning that it rolls back all modifications if the AppTx returns an error. This was implemented as a wrapper in basecoin, but it is a reasonable thing to support in the DB interface itself (especially since the implementation becomes quite non-trivial as soon as you support range queries).

To give a bit more reference to this concept in practice, read about [Savepoints in Postgresql](https://www.postgresql.org/docs/current/static/tutorial-transactions.html) ([reference](https://www.postgresql.org/docs/current/static/sql-savepoint.html)) or [Nesting transactions in SQL Server](http://dba-presents.com/index.php/databases/sql-server/43-nesting-transactions-and-save-transaction-command) (TL;DR: scroll to the bottom, section "Real nesting transactions with SAVE TRANSACTION")

### Functionality

Merkle trees work with key-value pairs, so we should most importantly focus on the basic Key-Value operations.  That is `Get`, `Set`, and `Remove`. We also need to return a merkle proof for any key, along with a root hash of the tree for committing state to the blockchain. This is just the basic merkle-tree stuff.

If it is possible with the implementation, it is nice to provide access to Range Queries.  That is, return all values where the key is between X and Y.  If you construct your keys wisely, it is possible to store lists (1:N) relations this way.  Eg, storing blog posts and the key is blog:`poster_id`:`sequence`, then I could search for all blog posts by a given `poster_id`, or even return just posts 10-19 from the given poster.

The construction of a tree that supports range queries was one of the [design decisions of go-merkle](https://github.com/tendermint/go-merkle/blob/master/README.md).  It is also kind of possible with [ethereum's patricia trie](https://github.com/ethereum/wiki/wiki/Patricia-Tree) as long as the key is less than 32 bytes.

In addition to range queries, there is one more nice feature that we could add to our data store - listening to events. Depending on your context, this is "reactive programming", "event emitters", "notifications", etc... But the basic concept is that a client can listen for all changes to a given key (or set of keys), and receive a notification when this happens. This is very important to avoid [repeated polling and wasted queries](http://resthooks.org/) when a client simply wants to [detect changes](https://www.rethinkdb.com/blog/realtime-web/).

If the database provides access to some "listener" functionality, the app can choose to expose this to the external client via websockets, web hooks, http2 push events, android push notifications, etc, etc etc.... But if we want to support modern client functionality, let's add support for this reactive paradigm in our DB interface.

**TODO** support for more advanced backends, eg. Bolt....

### Go Interface

I will start with a simple go interface to illustrate the in-process interface.  Once there is agreement on how this looks, we can work out the gRPC bindings to support calling out of process. These interfaces are not finalized code, but I think the demonstrate the concepts better than text and provide a strawman to get feedback.

```
// DB represents the committed state of a merkle-ized key-value store
type DB interface {
  // Snapshot returns a reference to last committed state to use for
  // providing proofs, you must close it at the end to garbage collect
  // the historical state we hold on to to make these proofs
  Snapshot() Prover

  // Start a transaction - only way to change state
  // This will return an error if there is an open Transaction
  Begin() (Transaction, error)

  // These callbacks are triggered when the Transaction is Committed
  // to the DB.  They can be used to eg. notify clients via websockets when
  // their account balance changes.
  AddListener(key []byte, listener Listener)
  RemoveListener(listener Listener)
}

// DBReader represents a read-only connection to a snapshot of the db
type DBReader interface {
  // Queries on my local view
  Has(key []byte) (bool, error)
  Get(key []byte) (Model, error)
  GetRange(start, end []byte, ascending bool, limit int) ([]Model, error)
  Closer
}

// Prover is an interface that lets one query for Proofs, holding the
// data at a specific location in memory
type Prover interface {
  DBReader

  // Hash is the AppHash (RootHash) for this block
  Hash() (hash []byte)

  // Prove returns the data along with a merkle Proof
  // Model and Proof are nil if not found
  Prove(key []byte) (Model, Proof, error)
}

// Transaction is a set of state changes to the DB to be applied atomically.
// There can only be one open transaction at a time, which may only have
// maximum one subtransaction at a time.
// In short, at any time, there is exactly one object that can write to the
// DB, and we can use Subtransactions to group operations and roll them back
// together (kind of like `types.KVCache` from basecoin)
type Transaction interface {
  DBReader

  // Change the state - will raise error immediately if this Transaction
  // is not holding the exclusive write lock
  Set(model Model) (err error)
  Remove(key []byte) (removed bool, err error)

  // Subtransaction starts a new subtransaction, rollback will not affect the
  // parent.  Only on Commit are the changes applied to this transaction.
  // While the subtransaction exists, no write allowed on the parent.
  // (You must Commit or Rollback the child to continue)
  Subtransaction() Transaction

  // Commit this transaction (or subtransaction), the parent reference is
  // now updated.
  // This only updates persistant store if the top level transaction commits
  // (You may have any number of nested sub transactions)
  Commit() error

  // Rollback ends the transaction and throw away all transaction-local state,
  // allowing the tree to prune those elements.
  // The parent transaction now recovers the write lock.
  Rollback()
}

// Listener registers callbacks on changes to the data store
type Listener interface {
  OnSet(key, value, oldValue []byte)
  OnRemove(key, oldValue []byte)
}

// Proof represents a merkle proof for a key
type Proof interface {
  RootHash() []byte
  Verify(key, value, root []byte) bool
}

type Model interface {
  Key() []byte
  Value() []byte
}

// Closer releases the reference to this state, allowing us to garbage collect
// Make sure to call it before discarding.
type Closer interface {
  Close()
}
```

### Remote Interface

The use-case of allowing out-of-process calls is very powerful.  Not just to provide a powerful merkle-ready data store to non-go applications.

It we allow the ABCi app to maintain the only writable connections, we can guarantee that all transactions are only processed through the tendermint consensus engine.  We could then allow multiple "web server" machines "read-only" access and scale out the database reads, assuming the consensus engine, ABCi logic, and public key cryptography is more the bottleneck than the database. We could even place the consensus engine, ABCi app, and data store on one machine, connected with unix sockets for security, and expose a tcp/ssl interface for reading the data, to scale out query processing over multiple machines.

But returning our focus directly to the ABCi app (which is the most important use case).  An app may well want to maintain 100 or 1000 snapshots of different heights to allow people to easily query many proofs at a given height without race conditions (very important for IBC, ask Jae).  Thus, we should not require a separate TCP connection for each height, as this gets quite awkward with so many connections. Also, if we want to use gRPC, we should consider the connections potentially transient (although they are more efficient with keep-alive).

Thus, the wire encoding of a transaction or a snapshot should simply return a unique id.  All methods on a `Prover` or `Transaction` over the wire can send this id along with the arguments for the method call.  And we just need a hash map on the server to map this id to a state.

The only negative of not requiring a persistent tcp connection for each snapshot is there is no auto-detection if the client crashes without explicitly closing the connections.  Thus, I would suggest adding a `Ping` thread in the gRPC interface which keeps the Snapshot alive.  If no ping is received within a server-defined time, it may automatically close those transactions.  And if we consider a client with 500 snapshots that needs to ping each every 10 seconds, that is a lot of overhead, so we should design the ping to accept a list of IDs for the client and update them all.  Or associate all snapshots with a clientID and then just send the clientID in the ping.  (Please add other ideas on how to detect client crashes without persistent connections).

To encourage adoption, we should provide a nice client that uses this gRPC interface (like we do with ABCi).  For go, the client may have the exact same interface as the in-process version, just that the error call may return network errors, not just illegal operations.  We should also add a client with a clean API for Java, since that seems to be popular among app developers in the current tendermint community.  Other bindings as we see the need in the server space.

### Persistence

Any data store worth it's name should not lose all data on a crash.  Even [redis provides some persistence](https://redis.io/topics/persistence) these days. Ideally, if the system crashes and restarts, it should have the data at the last block N that was committed.  If the system crash during the commit of block N+1, then the recovered state should either be block N or completely committed block N+1, but no partial state between the two.  Basically, the commit must be an atomic operation (even if updating 100's of records).

To avoid a lot of headaches ourselves, we can use an existing data store, such as leveldb, which provides `WriteBatch` to group all operations.

The other issue is cleaning up old state.  We cannot delete any information from our persistent store, as long as any snapshot holds a reference to it (or else we get some panics when the data we query is not there).  So, we need to store the outstanding deletions that we can perform when the snapshot is `Close`d.  In addition, we must consider the case that the data store crashes with open snapshots.  Thus, the info on outstanding deletions must also be persisted somewhere.  Something like a "delete-behind log" (the opposite of a "write ahead log").

This is not a concern of the generic interface, but each implementation should take care to handle this well to avoid accumulation of unused references in the data store and eventual data bloat.

#### Backing stores

It is way outside the scope of this project to build our own database that is capable of efficiently storing the data, provide multiple read-only snapshots at once, and save it atomically. The best approach seems to select an existing database (best a simple one) that provides this functionality and build upon it, much like the current `go-merkle` implementation builds upon `leveldb`.  After some research here are winners and losers:

**Winners**

* Leveldb - [provides consistent snapshots](https://ayende.com/blog/161705/reviewing-leveldb-part-xiii-smile-and-here-is-your-snapshot), and [provides tooling for building ACID compliance](http://codeofrob.com/entries/writing-a-transaction-manager-on-top-of-leveldb.html)
  * Note there are at least two solid implementations available in go - [goleveldb](https://github.com/syndtr/goleveldb) - a pure go implementation, and [levigo](https://github.com/jmhodges/levigo) - a go wrapper around leveldb.
  * Goleveldb is much easier to compile and cross-compile (not requiring cgo), while levigo (or cleveldb) seems to provide a significant performance boosts (but I had trouble even running benchmarks)
* PostgreSQL - fully supports these ACID semantics if you call `SET TRANSACTION ISOLATION LEVEL SERIALIZABLE` at the beginning of a transaction (tested)
  * This may be total overkill unless we also want to make use of other features, like storing data in multiple columns with secondary indexes.
  * Trillian can show an example of [how to store a merkle tree in sql](https://github.com/google/trillian/blob/master/storage/mysql/tree_storage.go)

**Losers**

* Bolt - open [read-only snapshots can block writing](https://github.com/boltdb/bolt/issues/378)
* Mongo - [barely even supports atomic operations](https://docs.mongodb.com/manual/core/write-operations-atomicity/), much less multiple snapshots

**To investigate**

* [Trillian](https://github.com/google/trillian) - has a [persistent merkle tree interface](https://github.com/google/trillian/blob/master/storage/tree_storage.go) along with [backend storage with mysql](https://github.com/google/trillian/blob/master/storage/mysql/tree_storage.go), good inspiration for our design if not directly using it
* [Moss](https://github.com/couchbase/moss) - another key-value store in go, seems similar to leveldb, maybe compare with performance tests?

### Security

When allowing access out-of-process, we should provide different mechanisms to secure it.  The first is the choice of binding to a local unix socket or a tcp port.  The second is the optional use of ssl to encrypt the connection (very important over tcp).  The third is authentication to control access to the database.

We may also want to consider the case of two server connections with different permissions, eg. a local unix socket that allows write access with no more credentials, and a public TCP connection with ssl and authentication that only provides read-only access.

The use of ssl is quite easy in go, we just need to generate and sign a certificate, so it is nice to be able to disable it for dev machines, but it is very important for production.

For authentication, let me sketch out a minimal solution. The server could just have a simple config file with key/bcrypt(password) pairs along with read/write permission level, and read that upon startup.  The client must provide a username and password in the HTTP headers when making the original HTTPS gRPC connection.

This is super minimal to provide some protection. Things like LDAP, OAuth and single-sign on seem overkill and even potential security holes.  Maybe there is another solution somewhere in the middle.
