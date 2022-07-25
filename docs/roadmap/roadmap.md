---
order: 1
---

# Tendermint Roadmap

*Last Updated: Friday 8 October 2021*

This document endeavours to inform the wider Tendermint community about development plans and priorities for Tendermint Core, and when we expect features to be delivered. It is intended to broadly inform all users of Tendermint, including application developers, node operators, integrators, and the engineering and research teams.

Anyone wishing to propose work to be a part of this roadmap should do so by opening an [issue](https://github.com/tendermint/tendermint/issues/new/choose) in the spec. Bug reports and other implementation concerns should be brought up in the [core repository](https://github.com/tendermint/tendermint).

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

An overhaul of the existing interface between the application and consensus, to give the application more control over block construction. ABCI++ adds new hooks allowing modification of transactions before they get into a block, verification of a block before voting, injection of signed information into votes, and more compact delivery of blocks after agreement (to allow for concurrent execution). [More](https://github.com/tendermint/tendermint/blob/v0.35.x/rfc/004-abci%2B%2B.md)

### Proposer-Based Timestamps

Proposer-based timestamps are a replacement of [BFT time](https://docs.tendermint.com/master/spec/consensus/bft-time.html), whereby the proposer chooses a timestamp and validators vote on the block only if the timestamp is considered *timely*. This increases reliance on an accurate local clock, but in exchange makes block time more reliable and resistant to faults. This has important use cases in light clients, IBC relayers, CosmosHub inflation and enabling signature aggregation. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-071-proposer-based-timestamps.md)

### Soft Upgrades

We are working on a suite of tools and patterns to make it easier for both node operators and application developers to quickly and safely upgrade to newer versions of Tendermint. [More](https://github.com/tendermint/spec/pull/222)

### Minor Works

- Remove the "legacy" P2P framework, and clean up of P2P package. [More](https://github.com/tendermint/tendermint/issues/5670)
- Remove the global mutex from the local ABCI client to enable application-controlled concurrency. [More](https://github.com/tendermint/tendermint/issues/7073)
- Enable P2P support for light clients
- Node orchestration of services + Node initialization and composibility
- Remove redundancy in several data structures. Remove unused components such as the block sync v2 reactor, gRPC in the RPC layer, and the socket-based remote signer.
- Improve node visibility by introducing more metrics

## V0.37 (expected Q3 2022)

### Complete P2P Refactor

Finish the final phase of the P2P system. Ongoing research and planning is taking place to decide whether to adopt [libp2p](https://libp2p.io/), alternative transports to `MConn` such as [QUIC](https://en.wikipedia.org/wiki/QUIC) and handshake/authentication protocols such as [Noise](https://noiseprotocol.org/). Research into more advanced gossiping techniques.

### Streamline Storage Engine

Tendermint currently has an abstraction to allow support for multiple database backends. This generality incurs maintenance overhead and interferes with application-specific optimizations that Tendermint could use (ACID guarantees, etc.). We plan to converge on a single database and streamline the Tendermint storage engine. [More](https://github.com/tendermint/tendermint/pull/6897)

### Evaluate Interprocess Communication

Tendermint nodes currently have multiple areas of communication with other processes (ABCI, remote-signer, P2P, JSONRPC, websockets, events as examples). Many of these have multiple implementations in which a single suffices. Consolidate and clean up IPC. [More](https://github.com/tendermint/tendermint/blob/master/docs/rfc/rfc-002-ipc-ecosystem.md)

### Minor Works

- Amnesia attack handling. [More](https://github.com/tendermint/tendermint/issues/5270)
- Remove / Update Consensus WAL. [More](https://github.com/tendermint/tendermint/issues/6397)
- Signature Aggregation. [More](https://github.com/tendermint/tendermint/issues/1319)
- Remove gogoproto dependency. [More](https://github.com/tendermint/tendermint/issues/5446)

## V1.0 (expected Q4 2022)

Has the same feature set as V0.37 but with a focus towards testing, protocol correctness and minor tweaks to ensure a stable product. Such work might include extending the [consensus testing framework](https://github.com/tendermint/tendermint/issues/5920), the use of canary/long-lived testnets and greater integration tests.

## Post 1.0 Work

- Improved block propagation with erasure coding and/or compact blocks. [More](https://github.com/tendermint/spec/issues/347)
- Consensus engine refactor
- Bidirectional ABCI
- Randomized Leader Election
- ZK proofs / other cryptographic primitives
- Multichain Tendermint
