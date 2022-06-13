---
order: 1
---

# Tendermint Roadmap

*Last Updated: Friday 4 February 2022*

This document endeavours to inform the wider Tendermint community about development plans and priorities for Tendermint Core, and when we expect features to be delivered. It is intended to broadly inform all users of Tendermint, including application developers, node operators, integrators, and the engineering and research teams.

Anyone wishing to propose work to be a part of this roadmap should do so by opening an [issue](https://github.com/tendermint/tendermint/issues/new/choose). Bug reports and other implementation concerns should be brought up in the [core repository](https://github.com/tendermint/tendermint).

This roadmap should be read as a high-level guide to plans and priorities, rather than a commitment to schedules and deliverables. Features earlier on the roadmap will generally be more specific and detailed than those later on. We will update this document periodically to reflect the current status.

The upgrades are split into two components: **Epics**, the features that define a release and to a large part dictate the timing of releases; and **minors**, features of smaller scale and lower priority, that could land in neighboring releases.

## V0.35 (completed Q3 2021)

### Prioritized Mempool

Transactions were previously added to blocks in the order with which they arrived to the mempool. Adding a priority field via `CheckTx` gives applications more control over which transactions make it into a block. This is important in the presence of transaction fees. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-067-mempool-refactor.md)

### Refactor of the P2P Framework

The Tendermint P2P system is undergoing a large redesign to improve its performance and reliability. The first phase of this redesign is included in 0.35. This phase cleans and decouples abstractions, improves peer lifecycle management, peer address handling and enables pluggable transports. It is implemented to be protocol-compatible with the previous implementation. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-062-p2p-architecture.md)

### State Sync Improvements

Following the initial version of state sync, several improvements have been made. These include the addition of [Reverse Sync](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-068-reverse-sync.md) needed for evidence handling, the introduction of a [P2P State Provider](https://github.com/tendermint/tendermint/pull/6807) as an alternative to RPC endpoints, new configuration parameters to adjust throughput, and several bug fixes.

### Custom event indexing + PSQL Indexer

Added a new `EventSink` interface to allow alternatives to Tendermint's proprietary transaction indexer. We also added a PostgreSQL Indexer implementation, allowing rich SQL-based index queries. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-065-custom-event-indexing.md)

### Minor Works

- Several Go packages were reorganized to make the distinction between public APIs and implementation details more clear.
- Block indexer to index begin-block and end-block events. [More](https://github.com/tendermint/tendermint/pull/6226)
- Block, state, evidence, and light storage keys were reworked to preserve lexicographic order. This change requires a database migration. [More](https://github.com/tendermint/tendermint/pull/5771)
- Introduciton of Tendermint modes. Part of this change includes the possibility to run a separate seed node that runs the PEX reactor only. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-052-tendermint-mode.md)

## V0.36 (expected Q1 2022)

### ABCI++

An overhaul of the existing interface between the application and consensus, to give the application more control over block construction. ABCI++ adds new hooks allowing modification of transactions before they get into a block, verification of a block before voting, and complete delivery of blocks after agreement (to allow for concurrent execution). It enables both immediate and delayed agreement. [More](https://github.com/tendermint/tendermint/blob/master/spec/abci++/README.md)

### Proposer-Based Timestamps

Proposer-based timestamps are a replacement of [BFT time](https://github.com/tendermint/tendermint/blob/master/spec/consensus/bft-time.md), whereby the proposer chooses a timestamp and validators vote on the block only if the timestamp is considered *timely*. This increases reliance on an accurate local clock, but in exchange makes block time more reliable and resistant to faults. This has important use cases in light clients, IBC relayers, CosmosHub inflation and enabling signature aggregation. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-071-proposer-based-timestamps.md)

### RPC Event Subscription

The websocket-based RPC event subscription API has been an ongoing pain point for users and operators of Tendermint. In this release, we are adding a new API for event subscription that will be more predictable and reliable for clients, easier to use, and reduce resource pressure for the consensus node. The existing API based on websockets will be kept as-is but deprecated, and we plan to remove it entirely in the following release.  [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-075-rpc-subscription.md)

### Minor Works

- Remove the "legacy" P2P framework, and clean up of P2P package. [More](https://github.com/tendermint/tendermint/issues/5670)
- Remove the global mutex from the local ABCI client to enable application-controlled concurrency. [More](https://github.com/tendermint/tendermint/issues/7073)
- Improve life cycle management of a node and its reactors.
- Remove redundancy in several data structures. Remove unused components such as the block sync v2 reactor, gRPC in the RPC layer, and the socket-based remote signer.
- Improve node visibility through the introduction of more metrics
- Migrating locally configured consensus timeouts to global consensus parameters. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-074-timeout-params.md)

## V0.37 (expected Q3 2022)

### LibP2P Implementation

Implement LibP2P to replace `mconnection` in sending and receiving messages across `Channel`s. Use LibP2P also for peer life cycle management and discovery. This aims to reduce the occurence of network thrashing and overall network traffic to provide a more stable networking layer. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-073-libp2p.md).

### Soft Upgrades

We are working on a suite of tools and patterns to make it easier for both node operators and application developers to quickly and safely upgrade to newer versions of Tendermint. [More](https://github.com/tendermint/spec/pull/222)

### Streamline Storage Engine

Tendermint currently has an abstraction to allow support for multiple database backends. This generality incurs maintenance overhead and interferes with application-specific optimizations that Tendermint could use (ACID guarantees, etc.). We plan to converge on a single database and streamline the Tendermint storage engine. [More](https://github.com/tendermint/tendermint/pull/6897)

### Minor Works

- Amnesia attack handling. [More](https://github.com/tendermint/tendermint/issues/5270)
- Remove / Update Consensus WAL. [More](https://github.com/tendermint/tendermint/issues/6397)
- Signature Aggregation. [More](https://github.com/tendermint/tendermint/issues/1319)
- Remove gogoproto dependency. [More](https://github.com/tendermint/tendermint/issues/5446)
- Enable P2P support for light clients. [More](https://github.com/tendermint/tendermint/blob/master/docs/rfc/rfc-010-p2p-light-client.rst)

## V1.0 (expected Q4 2022)

Has the same feature set as V0.37 but with a focus towards testing, protocol correctness and minor tweaks to ensure a stable product. Such work might include extending the [consensus testing framework](https://github.com/tendermint/tendermint/issues/5920), the use of canary/long-lived testnets and greater integration tests.

## Post 1.0 Work

- Improved block propagation with erasure coding and/or compact blocks. [More](https://github.com/tendermint/tendermint/issues/7932)
- Consensus engine refactor
- Fork accountability protocol
- Bidirectional ABCI
- Randomized Leader Election
- ZK proofs / other cryptographic primitives
- Multichain Tendermint
