---
order: 1
parent:
  order: false
---

# Requests for Comments

A Request for Comments (RFC) is a record of discussion on an open-ended topic
related to the design and implementation of Tendermint Core, for which no
immediate decision is required.

The purpose of an RFC is to serve as a historical record of a high-level
discussion that might otherwise only be recorded in an ad hoc way (for example,
via gists or Google docs) that are difficult to discover for someone after the
fact. An RFC _may_ give rise to more specific architectural _decisions_ for
Tendermint, but those decisions must be recorded separately in [Architecture
Decision Records (ADR)](./../architecture).

As a rule of thumb, if you can articulate a specific question that needs to be
answered, write an ADR. If you need to explore the topic and get input from
others to know what questions need to be answered, an RFC may be appropriate.

## RFC Content

An RFC should provide:

- A **changelog**, documenting when and how the RFC has changed.
- An **abstract**, briefly summarizing the topic so the reader can quickly tell
  whether it is relevant to their interest.
- Any **background** a reader will need to understand and participate in the
  substance of the discussion (links to other documents are fine here).
- The **discussion**, the primary content of the document.

The [rfc-template.md](./rfc-template.md) file includes placeholders for these
sections.

## Table of Contents

- [RFC-000: P2P Roadmap](./rfc-000-p2p-roadmap.rst)
- [RFC-001: Storage Engines](./rfc-001-storage-engine.rst)
- [RFC-002: Interprocess Communication](./rfc-002-ipc-ecosystem.md)
- [RFC-003: Performance Taxonomy](./rfc-003-performance-questions.md)
- [RFC-004: E2E Test Framework Enhancements](./rfc-004-e2e-framework.rst)
- [RFC-005: Event System](./rfc-005-event-system.rst)
- [RFC-006: Event Subscription](./rfc-006-event-subscription.md)
- [RFC-007: Deterministic Proto Byte Serialization](./rfc-007-deterministic-proto-bytes.md)
- [RFC-008: Don't Panic](./rfc-008-do-not-panic.md)
- [RFC-009: Consensus Parameter Upgrades](./rfc-009-consensus-parameter-upgrades.md)
- [RFC-010: P2P Light Client](./rfc-010-p2p-light-client.rst)
- [RFC-011: Delete Gas](./rfc-011-delete-gas.md)
- [RFC-012: Event Indexing Revisited](./rfc-012-custom-indexing.md)
- [RFC-013: ABCI++](./rfc-013-abci++.md)
- [RFC-014: Semantic Versioning](./rfc-014-semantic-versioning.md)
- [RFC-015: ABCI++ Tx Mutation](./rfc-015-abci++-tx-mutation.md)
- [RFC-016: Node Architecture](./rfc-016-node-architecture.md)
- [RFC-017: ABCI++ Vote Extension Propagation](./rfc-017-abci++-vote-extension-propag.md)
- [RFC-018: BLS Signature Aggregation Exploration](./rfc-018-bls-agg-exploration.md)
- [RFC-019: Configuration File Versioning](./rfc-019-config-version.md)
- [RFC-020: Onboarding Projects](./rfc-020-onboarding-projects.rst)

<!-- - [RFC-NNN: Title](./rfc-NNN-title.md) -->
