# Merkle data stores - Frey's proposal

To allow the efficient creation of an ABCi app, tendermint wishes to provide a reference implemention of a key-value store that provides merkle proofs of the data.  These proofs then quickly allow the ABCi app to provide an apphash to the consensus engine, as well as a full proof to any client.

This is equivalent to building a database, and I would propose designing it from the API first, then looking how to implement this (or make an adaptor from the API to existing implementations). Once we agree on the functionality and the interface, we can implement the API bindings, and then work on building adaptors to existince merkle-ized data stores, or modifying the stores to support this interface

## TL;DR

* **Interface**
  * This interface should be domain-specific - ie. designed just for this use case
  * It should present a simple go interface for embedding the data store in-process
  * It should create a gRPC/protobuf API for calling from any client
  * It should provide and maintain client adaptors from our in-process interface to gRPC client calls for at least golang and java (maybe more languages?)
  * It should provide and maintain server adaptors from our gRPC calls to the in-process interface for golang at least (unless there is another server we wish to support)
* **State**
  * There are two concepts of state, "committed state" and "working state"
  * The working state is only accessible from the ABCi app, allows writing, but does not need to support proofs.
  * When we commit the "working state", it becomes a new "commmited state" and has an immutible root hash, provides proofs, and can be exposed to external clients.
* **Transactions**
  * The database always allows creating a read-only transaction at the last "committed state", this transaction can serve read queries and proofs.
  * The database maintains all data to serve these read transactions until they are closed by the client (or time out).  This allows the client(s) to determine how much old info is needed
  * The database can only support *maximal* one writable transaction at a time.  This makes it easy to enforce serializability, and attempting to start a second writeable transaction may trigger a panic.
* **Functionality**
  * It must support efficient key-value operations (get/set/delete)
  * It must support returning merkle proofs for any "committed state"
  * It should support range queries on subsets of the key space if possible (ie. if the db doesn't hash keys)
  * It should also support listening to changes to a desired key via pub-sub or similar method, so I can quickly notify you on a change to your balance without constant polling.
  * It may support other db-specific query types as an extension to this interface, as long as all specified actions maintain their meaning.
* **Persistence**
  * It must support atomic persistance upon committing a new block.  That is, upon crash recovery, the state is guaranteed to represent the state at the end of a complete block (along with a note of which height it was).
  * It must delay deletion of old data as long as there are open read-only transactions refering to it, thus we must maintain some sort of WAL to keep track of pending cleanup.
  * When a transaction is closed, or when we recover from a crash, it should clean up all no longer needed data to avoid memory/storage leaks.
* **Security and Auth**
  * If we allow connections over gRPC, we must consider this issues and allow both encyption (SSL), and some basic auth rules to provent undesired access to the DB
  * This is client-specific and does not need to be supported in the in-process, embeded version.

## Details

Here we go more in-depth in each of the sections, explaining the reasoning and more details on the desired behavior. This document is only the high-level architecture and should support multiple implementations.  When building out a specific implementation, a similar document should be provided for that repo, showing how it implements these concepts, and details about memory usage, storage, efficiency, etc.

### Interface

I will start with a simple go interface to illustrate the in-process interface.  Once there is agreement on how this looks, we can work out the gRPC bindings to support calling out of process.

The use-case of allowing out-of-process calls is very powerful.  It we allow the ABCi app to maintain the only writable connections, we can guarantee that all transactions are only processed through the tendermint consensus engine.  We could then allow multiple "web server" machines "read-only" access and scale out the database reads, assuming the consensus engine, ABCi logic, and public key cryptography is more the bottleneck than the database.

**TODO**

### State

**TODO**

### Transactions

**TODO**

Points to add:

* Subtransactions
* Rollback and commit
* Multiplexing various transactions over on stateless gRPC connection
* Ping-ing and timeouts


### Functionality

**TODO**

### Persistence

**TODO**

### Security

**TODO**
