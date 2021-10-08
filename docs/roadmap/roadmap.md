---
order: false
parent:
  title: Roadmap
  order: 7
---

# Tendermint Roadmap

*Last Updated: Friday 8 October*

This document endeavours to inform the wider Tendermint community on the planned work, what they encompass, their priorities and when they expect to be delivered. It should ideally reflect the interests of those same sets of users: application developers, node operators, integrators and finally of the engineering and research teams themselves. The first step for anyone wishing to propose work to be a part of this roadmap should do so by opening an [issue](https://github.com/tendermint/spec/issues/new/choose) in the spec.

Like any attempt to predict the future, this roadmap comes with a certain degree of flexibility with features closer to now having greater specificity and certainty than those later on. It will be, however, updated at a regular cadence to give the most accurate signal possible.

The upgrades are split into two components: the epics, features that define a release and to a large part dictate the timing of releases and minors, features of smaller scale that don't have significant priorities and could land in neighboring releases.

## V0.35 (completed Q3 2021)

### Prioritized Mempool

Transactions were previously added to blocks in the order with which transactions arrived to the mempool. Adding a priority field via `CheckTx` gives applications control on the mechanism which dictates which transactions make it into a block. This is ideal in the case of transaction fees. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-067-mempool-refactor.md)

### Refactor of the P2P Framework

The first phase of a large project to redesign the Tendermint P2P system is included in 0.35. This cleans and  decouples abstractions, improves peer lifecycle management, peer address handling and enables pluggable transports. It is implemented to be backwards compatible. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-062-p2p-architecture.md)

### State Sync Improvements

Following the initial version of state sync, several improvements have been made. The addition of [Reverse Sync](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-068-reverse-sync.md) needed for evidence handling, the introduction of a [P2P State Provider](https://github.com/tendermint/tendermint/pull/6807) as an alternative to using RPC endpoints, new config parameters to adjust throughput and several fixes.

### Custom event indexing + PSQL Indexer

To remove dependency on Tendermint's proprietary transaction indexer an `EventSink` abstraction was created. This allowed for a PostgreSQL Indexer to be implemented offering rich query engine support to transactions. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-065-custom-event-indexing.md)

### Minor Works

- Internalization of API's to scope the public surface
- Block indexer to index begin block and end block events. [More](https://github.com/tendermint/tendermint/pull/6226)
- Order-preserving varint key encoding. Requires database migration. [More](https://github.com/tendermint/tendermint/pull/5771)
- Separate seed node

## V0.36 (expected Q1 2022)

### ABCI++

An overhaul of the existing interface to the application in order to unlock greater use cases. The changes add more hooks into consensus allowing for modification of transactions before they get into a block, verification of a block before voting, the ability to add signed information into votes and the compaction of delivering the block after agreement to allow for concurrent execution. [More](https://github.com/tendermint/spec/blob/master/rfc/004-abci%2B%2B.md)

### Proposed Based Timestamps

Replacement of BFT time whereby the proposer proposes a time and validators vote on the block only if the timestamp is considered *timely*. This increases reliance on an accurate local clock but makes block time more reliable and resistant to faults. This has important use cases in light clients, IBC relayers, CosmosHub inflation and enabling signature aggregation. [More](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-071-proposer-based-timestamps.md)

### Soft Upgrades

A hallmark of stable, reliable software is when the continual development doesn't put strain on the continual use. Tendermint aims to implement a suite of tools and patterns to allow node operators and application developers to seamless transition from one version to another. [More](https://github.com/tendermint/spec/pull/222)

### Minor Works

- Removal of Legacy Framework and clean up of the P2P package. [More](https://github.com/tendermint/tendermint/issues/5670)
- Remove global mutex around local client to enable application controlled concurrency. [More](https://github.com/tendermint/tendermint/issues/7073)
- Enable P2P support for light clients
- Node orchestration of services + Node initialization and composibility
- Remove redundancy in several data structures. Remove unused components.

## V0.37 (expected Q3 2022)

### Complete P2P Refactor

Finish the final phase of the P2P system. Ongoing research and planning is taking place to decide whether to adopt [libp2p](https://libp2p.io/), alternative transports to `MConn` such as [QUIC](https://en.wikipedia.org/wiki/QUIC) and handshake/authentication protocols such as [Noise](https://noiseprotocol.org/). Research into more advanced gossiping techniques.

### Streamline Storage Engine

Tendermint has a current abstraction to allow support for multiple databases. This incurs more maintenance and the generalization required forgoes any optimizations that Tendermint can make (ACID guarantees etc..). We plan to converge on a single database and streamline the Tendermint storage engine. [More](https://github.com/tendermint/tendermint/pull/6897)

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
- Conensus engine refactor (pipelining)
- Bidirectional ABCI
- ZK proofs / other cryptographic primatives
- Multichain Tendermint
