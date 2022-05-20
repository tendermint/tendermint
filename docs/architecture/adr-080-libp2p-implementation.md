# ADR 80: Libp2p Implementation Plan

## Changelog

- 2022-05-20: Initial Draft.

## Status

Accepted

## Context

Adopting libp2p is an larger project that is the culmination of a long
programme of work in the tendermint codebase.

## Alternative Approaches

None. 

The decision to adopt libp2p was part of ADR 73, and this ADR lays out
an implementation plan and records a collection of decisions related
to this implementation which have not been formally collected in any
other form. 

## Decisions

### Feature Parity

The goal of this project should be a p2p stack that has feature parity
with the legacy stack. Everything that is possible with the p2p stack
today should be possible in the future, eventually reducing the
overhead of peer communication and complexity in the gossip/peer layer
of the system. 

In persuit of this goal, this project has two explicit *non-goals*:

- Do not support message/wire compatibility with legacy stack.
- Do not support dual-mode (legacy/libp2p) in a single node or network.

There does not seem to be a compelling use case of having a release
that contains both P2P stacks. While either of these goals could
be theoretically possible, by eliminating both of these goals we can
effectively reduce the scope of the project and increase our
confidence.

### Target Iterative Progress 

Rather than produce an extensive design document (other than this
record of decisions,) we will work iteratively in a topic 
branch--`main/libp2p`--to prototype the new libp2p integration. This
work will aim to: 

- develop a working (by way of the e2e tests), implementation of the
  p2p layer without requiring changes to the reactor implementations.
- migrate the reactor tests after completing a prototype. 
- address design questions as they come up, by amending this document
  or in periodic conversations over patches. 
- minimize or avoid any interface changes between the reactor
  implementations and the peer layer. 

### Use libp2p Idiomatically 



### Minimize P2P Code

One of the largest benefits of this project is that we should be able
to delete the vast majority of the existing p2p code, which should
reduce the maintenance burden within Tendermint.

### Defer Transport Decisions

There are many high level aspects of p2p systems such as handshakes between
nodes, transport protocols, which are fully configurable in the
context of libp2p. There is no need to make decisions about these at
this time, our decisions. 

We should not, however, (necessarily) aim to delegate these decisions
to the users. While we should make it possible for users to pass a
custom libp2p connection object (`host.Host`) with limited support, we
should **not** aim to offer too many customizations or configurations
of different libp2p fundamentals.

### Use GossipSub for Gossip Communication 

An unfortunate consequence of the current p2p implementation is that
reactors (e.g. mempool, consensus, and evidence) implement their own
gossip logic, and maintain their own tracking of the connected peer
network. These workloads should eventually migrate to using gosipsub
(e.g. setting `broadcats: true` and letting libp2p broadcast,)
however, this requires removing all of the legacy code. In the short
term, we should leave the legacy gossip implementation for the first
phase of the project. 

## Detailed Design

Based on what we currently know about this process, we should aim to: 

- complete basic implementation of P2P fundamentals (e.g. Channel,
  host connection, etc.) for libp2p with the goal of having e2e
  support.
  
- add support for running libp2p e2e test networks, and add this as a
  branch in the test configuration and enable nightly testing of the
  libp2p branch. 

- migrate all unit tests away from using the legacy router, and engage
  in some design work on some kind of new test harness. Consider
  reviving a shimed routing layer or using libp2p's features, for
  testing purposes. It might also be possible to actually use real
  networks for testing.

- delete all legacy p2p code. 

- migrate all gossip workloads to use gossipsub. 

- fine-tune settings on default libp2p network configurations.

- run test nets to improve confidence and support tuning.

## Consequences

### Positive

The approach here is lightweight and avoids the trap of
over-engineering the process or converging on an implementation that
is hard to get correct. The iterative approach will let us make
progress pragmatically. 

### Negative

Without more formal design documentation, some decisions will be
difficult to anticipate and it will be hard to get context for these
changes without a familiarity with the existing system, although even
thoroughly designed solutions may also suffer from this weakness. 

Additionally, the iterative approach may lead tendermint initially
recapitulate some of the problems with the legacy P2P design on top of
the libp2p infrastructure: this should be ameliorated by the
additional flexibility. 

### Neutral

None.

## References

- [ADR 73: Libp2p][adr73]
- [ADR 61: P2P Refactor Scope][adr61]
- [ADR 62: P2P Architecture][adr62]
- [P2P Roadmap RFC][rfc]

[adr73]: ./adr-073-libp2p.md
[adr61]: ./adr-061-p2p-refactor-scope.md
[adr62]: ./adr-062-p2p-architecture.md
[rfc]: ../rfc/rfc-000-p2p-roadmap.rst

