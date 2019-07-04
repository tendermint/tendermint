# ADR 042: State Sync Design

## Changelog

2019-06-27: Init by EB
2019-07-04: Follow up by brapse

## Context
StateSync is a feature which would allow a new node to receive a
snapshot of the application state without downloading blocks or going
through consensus. Once downloaded, the node could switch to FastSync
and eventually participate in consensus. The goal of StateSync is to
facilitate setting up a new node as quickly as possible.

## Considerations
Because Tendermint doesn't know anything about the application state,
the goal of StateSync of to broker messages between nodes and through
the ABCI. The implementation will have multiple touch points through the
codebase:

* A StateSync reactor to facilitate peer communication
* A Set of ABCI messages to transmit application state to the reactor
* A Set of MultiStore APIs for exposing snapshot data to the ABCI
* A Storage format with validation and performance conciderations

### Implementation Properties
Beyond the approach, any implementation of StateSync can be evaluated
across different criteria:

* Speed: Expected throughput of producing and consuming snapshots
* Safety: Cost of pushing partial or completely invalid snapshots to a node
* Effort: How much effort does an implementation require

### Implementation Question

* What is the format of a snapshot
    * Complete snapshot 
    * Ordered IAVL key ranges
    * Compressed individually chunks which can be validated
* How is data validated
    * Trust a peer with it's data blindly
    * Trust a majority of peers
    * Use light client validation to validate any key based on consensus
      produced merkle tree root
* What are the performance characteristics
    * Random vs sequential reads
    * How parallelizeable is the scheduling algorithm

### Proposals
Broadly speaking there are two approaches to this problem which have had
varying degrees of discussion and progress. These approach can be
summarized as:

* Lazy: Where snapshots are produced dynamically at request time
* Eager: Where snapshots are produced periodically and served from disk at request time

#### Lazy StateSync
An [initial specification](https://docs.google.com/document/d/15MFsQtNA0MGBv7F096FFWRDzQ1vR6_dics5Y49vF8JU/edit?ts=5a0f3629) was published by Alexis Sellier.
In this design, the state has a given `size` of primitive elements (like
keys or nodes), each element is assigned a number from 0 to `size-1`,
and chunks consists of a range of such elements.  Ackratos raised
[some concerns](https://docs.google.com/document/d/1npGTAa1qxe8EQZ1wG0a0Sip9t5oX2vYZNUDwr_LVRR4/edit)
about this design, somewhat specific to the IAVL tree, and mainly concerning
performance of random reads and of iterating through the tree to determine element numbers
(ie. elements aren't indexed by the element number).

This design could be generalized to have numbered chunks instead of
elements, where chunks are indexed by their chunk number and determined
by some algorithm for grouping the elements in the tree. This would
allow pre-computation and persistence of the chunks that could alleviate
the concerns about real-time iteration and random reads.  However, there
seems to be a larger issue with the "numbered chunks" approach: it does
not seem possible, in general, to verify that the chunk corresponds to
the chunk number, even if it can be verified that the chunk is a valid
set of nodes in the tree. A malicious peer could provide a chunk that
contains valid nodes from the tree but does not actually match the
alleged chunk number. This could make it difficult for nodes to figure
out which parts of the tree they have and which parts they still need.
Hence, to go with this chunking approach to state sync, information
about how state is chunked would need to be hashed into the state
itself, or into the block header, so that chunk numbers could be
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

#### Eager StateSync
Warp Sync as implemented in Parity
["Warp Sync"](https://wiki.parity.io/Warp-Sync-Snapshot-Format.html) to rapidly
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

### Analysis of Lazy vs Eager

Lazy vs Eager have more in common than they differ. They all require
reactors on the tendermint side, a set of ABCI messages and a method for
serializing/deserializing snapshots facilitated by a SnapshotFormat.

The biggest difference between Lazy and Eager proposals is in the
read/write patterns necessitated by when serving a snapshot chunk.
Specifically, Lazy State Sync random reads to the underlying data
structure while Eager can optimize for sequential reads.

This distinctin between approaches was demonstrated by Binance's [ackratos](https://github.com/ackratos) in their
implementation of [Lazy State sync](https://github.com/tendermint/tendermint/pull/3243), The [analysis](https://docs.google.com/document/d/1npGTAa1qxe8EQZ1wG0a0Sip9t5oX2vYZNUDwr_LVRR4/) of the performance, and
follow up implementation of [Warp Sync](http://github.com/tendermint/tendermint/pull/3594).

One proposed difference between Lazy and Eager state sync is the safety
model. It had been suggested that Lazy State Sync could use light client
validation while Warp Sync was unsuitable for public networks as it
relies on the majority of peers to produce a manifest. This manifest
would require complete syncing before validation of the data against the
merkle root were possible. This could create DDOS attack vector in which
a peer publishes an invalid manifest and making peers download invalid
data. For this reason, WarpSync is faster but less safety-efficient; It
as it takes longer to realize that data is invalid.

However, upon analysis of the approaches with the orthogonal axis of
eager/lazy and chunk/snapshot validation suggest that a version of
WarpSync with per chunk light client validation might get the optimal
performance and safety.

## Decision: WarpSync With Per Chunk Light Client Validation

The conclusion after thorough concideration of the
advantages/disadvances of eager/lazy is to produce a state sync which
replicates the performance IO characteristic of WarpSync and the security of
per chunk validation. Manifests would be queries from nodes but be
static across peers, providing a mapping state to pre-computed chunks
allowing the snapshot to be downloaded in parallel.

### Implementation

* A new StateSync reactor brokers message transmission between the peers
  and the ABCI application
* A set of ABCI Messages
* Design SnapshotFormat as an interface which can:
    * validate chunks
    * read/write chunks from file
    * read/write chunks to/from application state store
    * convert manifests into chunkRequest ABCI messages
* Implement SnapshotFormat for cosmos-hub with concrete implementation for:
    * read/write chunks in a way which can be:
        * parallelized accros peers
        * validated on receipt
    * read/write to/from IAVL+ tree

![StateSync Architecture Diagram](img/state-sync.png)

## Implementation Path

* Create StateSync reactor based on  [#3753](https://github.com/tendermint/tendermint/pull/3753)
* Design SnapshotFormat with an eye towards cosmos-hub implementation
* ABCI message to send/receive SnapshotFormat
* IAVL+ changes to support SnapshotFormat
* Deliver Warp sync (no chunk validation)
* light client implementation for weak subjectivity
* Deliver StateSync with chunk validation

## Status

Proposed

## Concequences

### Neutral

### Positive
* Safe & performant state sync design substantiated with real world implementation experience
* General interfaces allowing application specific innovation
* Parallizable implementation trajectory with reasonable engineering effort

### Negative
* Static Scheduling lacks opportunity for real time chunk availability optimizations

## References
[sync: Sync current state without full replay for Applications](https://github.com/tendermint/tendermint/issues/828) - original issue
[tendermint state sync proposal](https://docs.google.com/document/d/15MFsQtNA0MGBv7F096FFWRDzQ1vR6_dics5Y49vF8JU/edit?ts=5a0f3629) - Cloudhead proposal
[tendermint state sync proposal 2](https://docs.google.com/document/d/1npGTAa1qxe8EQZ1wG0a0Sip9t5oX2vYZNUDwr_LVRR4/edit) - ackratos proposal
[proposal 2 implementation](https://github.com/tendermint/tendermint/pull/3243)  - ackratos implementation
[WIP General/Lazy State-Sync pseudo-spec](https://github.com/tendermint/tendermint/issues/3639) - Jae Proposal
[Warp Sync Implementation](https://github.com/tendermint/tendermint/pull/3594) - ackratos


