# ADR 073: Adopt LibP2P

## Changelog

- 2021-11-02: Initial Draft (@tychoish)

## Status

Proposed. 

## Context

As part of the 0.35 development cycle, the Tendermint team completed
first phase of the work described in ADRs 61 and 62, which included a
large scale refactoring of the reactors and the p2p message
routing. This replaced the switch and many of the other legacy
components, without breaking protocol or network-level
interoperability and left the legacy connection/socket handling code. 

Following the release, the team has reexamined the state of the code
and the design, as well as Tendermints requirements for. The notes
from this process are available in the [P2P Roadmap
RFC](../rfc/rfc-000-p2p.rst).

While this ADR supersedes the decisions made in ADRs 60 and 61, it is
also enabled by the completed portions of this work. Previously, the
boundaries of peer management, message handling, and the higher level
business logic (e.g. "the reactors") were intermingled, and core
elements of the p2p system were responsible for the orchestration of
the higher level business logic. The completed aspects of the refactor
made it more obvious that the design implications of the legacy
components had outsized influence on the entire implementation, that
it would be difficult to iterate within the current abstractions, and
it would not be viable to maintain interoperability with legacy
systems while also achieving any of our broader objectives.

LibP2P is a thoroughly specificed implementation of a peer-to-peer
networking stack, designed specifically for systems such as
ours. Adopting LibP2P as the basis of the Tendermint will allow the
Tendermint team to focus more of their time on other differentiating
aspects of the system, and make it possible for the ecosystem as a
whole to take advantage of tooling and efforts of the LibP2P
platform.

## Alternative Approaches

As discussed in the related RFC, the primary alternative is to
continue development of tendermint's home grown peer-to-peer
layer. While this gives the Tendermint team a maximal level of control
over the peer system, the current peer system is unexceptional on its
own merits, and the prospective maintenance burden for this system
exceeds our tolerances for the medium term. 

It is also the case that Tendermint can and should differentiate
itself not on the basis of its networking implementation or peer
management tools, but on its consistent operator experience,
battletested consensus algorithm, and ergonomic user experience.

## Decision

Tendermint will adopt libp2p during the 0.37 development cycle,
replacing the bespoke Tendermint P2P stack. This will remove the
`Endpoint`, `Transport`, `Connection`, and `PeerManager` abstractions
and leave the reactors, `p2p.Router` and `p2p.Channel`
abstractions. 

LibP2P may obviate the need for a dedicated peer exchange (PEX)
reactor, which would also in turn obviate the need for a dedicated
seed mode. If this is the case, then all of this functionality would
be removed.

If it turns out, based on the advice of Protocol Labs, that it makes
sense to maintain separate pubsub or gossipsub topics
per-message-type, then the `Router` abstraction may dissolve
entirely.

## Detailed Design



## Open Questions

- Should all P2P traffic for a given node be pushed to a single topic,
  and we assume that a topic maps to a specific ChainID, or should
  each reactor (or type of message) have its own topic?
  
- Tendermint presently has a concept of message-type priority, which
  provides a very course QoS-like functionality and
  intuitively/theoretically ensures that evidence and consensus
  messages don't get starved by blocksync/statesync messages. It's
  unclear if we should attempt to replicate this with libp2p and even
  if we should.

- Is it possible to attach additional (and potentially arbitrary)
  information into the DHT as part of the heartbeats between nodes,
  such as the latest height, and then access that in arbitrary
  reactors. 
  
## Consequences

### Positive

- Reduce the maintenance burden for the Tendermint Core team by
  removing a large swath of legacy code that has proven to be
  difficult to modify safely.
  
- Provide users with a more stable peer and networking system,
  reducing the 

### Negative

- By deferring to library implementations 

- 

### Neutral

## References

